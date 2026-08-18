package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"frankenphp-manager/internal/app"
)

// ── add ──────────────────────────────────────────────────────────────────────

var addCmd = &cobra.Command{
	Use:   "add [directory]",
	Short: "Add a PHP project (interactive)",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		c, _ := app.LoadConfig()
		r := bufio.NewReader(os.Stdin)

		// Directory
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		} else {
			dir = prompt(r, "  Project directory", ".")
		}
		absDir, err := filepath.Abs(dir)
		must(err)
		if _, err := os.Stat(absDir); os.IsNotExist(err) {
			die("Directory not found: %s", absDir)
		}

		// Auto-detect
		if fw := app.DetectFramework(absDir); fw != "" {
			ok("Framework: %s", fw)
		}

		// Name
		defName := filepath.Base(absDir)
		name := prompt(r, "  Project name", defName)
		if _, exists := c.GetProject(name); exists {
			die("Project '%s' already exists", name)
		}

		// PHP version
		detectedPHP := app.DetectPHP(absDir)
		if detectedPHP != "" {
			ok("PHP from composer.json: %s", detectedPHP)
		}
		phpVer := prompt(r, "  PHP version (8.1-8.4)", orDefault(detectedPHP, "8.3"))
		if !validPHP(phpVer) {
			die("Unsupported: %s. Use: %v", phpVer, app.SupportedPHP)
		}

		// Port
		port := prompt(r, "  Port", app.NextPort(c))

		// Database
		dbName := ""
		if strings.ToLower(prompt(r, "  Create MySQL database? (Y/n)", "Y")) != "n" {
			dbName = strings.ReplaceAll(name, "-", "_")
			dbName = prompt(r, "  Database name", dbName)
		}

		// Redis
		useRedis := strings.ToLower(prompt(r, "  Enable Redis? (Y/n)", "Y")) != "n"

		// HTTPS mode
		color.Yellow("\n  HTTPS modes:")
		fmt.Println("    http       — HTTP only (local dev)")
		fmt.Println("    selfsigned — Self-signed HTTPS (local HTTPS testing)")
		fmt.Println("    acme       — Let's Encrypt ACME (production, needs domain)")
		httpMode := prompt(r, "  HTTPS mode", "http")
		if httpMode != "http" && httpMode != "selfsigned" && httpMode != "acme" {
			die("Invalid mode: %s. Use: http, selfsigned, acme", httpMode)
		}

		domain := ""
		acmeEmail := ""
		if httpMode == "acme" {
			domain = prompt(r, "  Domain (e.g. example.com)", "")
			if domain == "" {
				die("Domain is required for ACME mode")
			}
			acmeEmail = prompt(r, "  ACME email (for Let's Encrypt)", "")
			if acmeEmail == "" {
				die("Email is required for ACME/Let's Encrypt")
			}
		}

		// Network interface
		color.Yellow("\n  Network interface:")
		fmt.Println("    lo    — localhost only (127.0.0.1 + ::1)")
		fmt.Println("    eth0  — bind to eth0's IPs")
		fmt.Println("    any   — all interfaces (0.0.0.0 + [::])")
		fmt.Println()
		showInterfaces()
		fmt.Println()

		defIface := "lo"
		if httpMode == "acme" {
			defIface = app.DefaultInterface()
		}
		iface := prompt(r, "  Interface", defIface)

		// Resolve & show IPs
		ipv4, ipv6, err := app.ResolveInterface(iface)
		if err != nil {
			die("Interface error: %v", err)
		}
		info("Bind IPv4: %s", ipv4)
		if ipv6 != "" {
			info("Bind IPv6: %s", ipv6)
		}

		// Build
		p := &app.Project{
			Name: name, Dir: absDir, PHPVer: phpVer,
			Iface: iface, BindIPv4: ipv4, BindIPv6: ipv6,
			Port: port, DBName: dbName, UseRedis: useRedis,
			HTTPMode: httpMode, Domain: domain, ACMEEmail: acmeEmail,
		}

		info("Generating config...")
		must(app.GenerateProject(p, c))

		if dbName != "" {
			info("Creating database '%s'...", dbName)
			app.EnsureInfra(c)
			if _, err := app.ExecMySQL(c, "", fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", dbName)); err != nil {
				warn("DB create failed (MySQL may still be starting): %v", err)
			} else {
				ok("Database '%s'", dbName)
			}
		}

		c.AddProject(p)
		must(c.Save())

		info("Starting server...")
		if err := app.ProjectStart(name); err != nil {
			warn("Start failed: %v — try: frankenphp start %s", err, name)
		} else {
			ok("%s", projectURL(p))
		}

		fmt.Println()
		color.Green("✔ Project '%s' added!", name)
		fmt.Printf("  URL:     %s\n", projectURL(p))
		fmt.Printf("  Iface:   %s (%s)\n", iface, ipv4)
		fmt.Printf("  PHP:     %s\n", phpVer)
		fmt.Printf("  Dir:     %s\n", absDir)
		if dbName != "" {
			fmt.Printf("  DB:      %s\n", dbName)
		}
	},
}

// ── start ────────────────────────────────────────────────────────────────────

var startCmd = &cobra.Command{
	Use:   "start [project|all]",
	Short: "Start project(s)",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		c, _ := app.LoadConfig()
		must(app.EnsureInfra(c))
		forEachProject(c, args, func(p *app.Project) {
			if app.ProjectRunning(p.Name) {
				info("%s: already running", p.Name)
				return
			}
			info("Starting %s...", p.Name)
			if err := app.ProjectStart(p.Name); err != nil {
				warn("%s: %v", p.Name, err)
			} else {
				ok("%s → %s", p.Name, projectURL(p))
			}
		})
	},
}

// ── stop ─────────────────────────────────────────────────────────────────────

var stopCmd = &cobra.Command{
	Use:   "stop [project|all]",
	Short: "Stop project(s)",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		c, _ := app.LoadConfig()
		forEachProject(c, args, func(p *app.Project) {
			if !app.ProjectRunning(p.Name) {
				info("%s: not running", p.Name)
				return
			}
			info("Stopping %s...", p.Name)
			if err := app.ProjectStop(p.Name); err != nil {
				warn("%s: %v", p.Name, err)
			} else {
				ok("%s: stopped", p.Name)
			}
		})
	},
}

// ── restart ──────────────────────────────────────────────────────────────────

var restartCmd = &cobra.Command{
	Use:   "restart [project|all]",
	Short: "Restart project(s)",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		c, _ := app.LoadConfig()
		forEachProject(c, args, func(p *app.Project) {
			info("Restarting %s...", p.Name)
			app.ProjectStop(p.Name)
			if err := app.ProjectStart(p.Name); err != nil {
				warn("%s: %v", p.Name, err)
			} else {
				ok("%s → %s", p.Name, projectURL(p))
			}
		})
	},
}

// ── list ─────────────────────────────────────────────────────────────────────

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all projects",
	Run: func(cmd *cobra.Command, args []string) {
		c, _ := app.LoadConfig()
		projects := c.ProjectList()
		if len(projects) == 0 {
			warn("No projects. Run 'frankenphp add'")
			return
		}

		color.Cyan("\n  FrankenPHP Projects\n")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "  NAME\tPHP\tIFACE\tBIND\tPORT\tSTATUS\tHTTPS\tDB\n")
		fmt.Fprintf(w, "  ────\t───\t─────\t────\t────\t──────\t─────\t──\n")
		for _, p := range projects {
			st := color.RedString("stopped")
			if app.ProjectRunning(p.Name) {
				st = color.GreenString("running")
			}
			db := "—"
			if p.DBName != "" {
				db = p.DBName
			}
			https := "http"
			if p.HTTPMode == "acme" {
				https = color.GreenString("acme")
			} else if p.HTTPMode == "selfsigned" {
				https = color.YellowString("self")
			}
			iface := orDefault(p.Iface, "lo")
			bind := orDefault(p.BindIPv4, "—")
			if p.BindIPv6 != "" {
				bind = bind + "\n              " + p.BindIPv6
			}
			fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				p.Name, p.PHPVer, iface, bind, p.Port, st, https, db)
		}
		w.Flush()
		fmt.Println()
	},
}

// ── remove ───────────────────────────────────────────────────────────────────

var removeCmd = &cobra.Command{
	Use:     "rm [project]",
	Aliases: []string{"remove", "delete"},
	Short:   "Remove a project",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		c, _ := app.LoadConfig()
		name := args[0]
		p, exists := c.GetProject(name)
		if !exists {
			die("Project '%s' not found", name)
		}
		info("Stopping %s...", name)
		app.ProjectStop(name)
		info("Removing config...")
		app.RemoveProjectDir(name)
		if p.DBName != "" {
			info("Dropping database '%s'...", p.DBName)
			app.ExecMySQL(c, "", fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", p.DBName))
		}
		c.RemoveProject(name)
		must(c.Save())
		ok("Project '%s' removed", name)
	},
}

// ── helpers ──────────────────────────────────────────────────────────────────

func forEachProject(c *app.Config, args []string, fn func(*app.Project)) {
	name := "all"
	if len(args) > 0 {
		name = args[0]
	}
	if name == "all" {
		projects := c.ProjectList()
		if len(projects) == 0 {
			warn("No projects configured")
			return
		}
		for _, p := range projects {
			fn(p)
		}
	} else {
		p, exists := c.GetProject(name)
		if !exists {
			die("Project '%s' not found", name)
		}
		fn(p)
	}
}

func projectURL(p *app.Project) string {
	switch p.HTTPMode {
	case "acme":
		return fmt.Sprintf("https://%s (iface %s)", p.Domain, p.Iface)
	case "selfsigned":
		return fmt.Sprintf("https://%s:%s (self-signed, iface %s)", p.BindIPv4, p.Port, p.Iface)
	default:
		return fmt.Sprintf("http://%s:%s (iface %s)", p.BindIPv4, p.Port, p.Iface)
	}
}

func showInterfaces() {
	ifaces, err := app.ListInterfaces()
	if err != nil {
		return
	}
	fmt.Println("    Available interfaces:")
	for _, name := range ifaces {
		fmt.Printf("      • %s\n", name)
	}
}

func orDefault(val, def string) string {
	if val == "" {
		return def
	}
	return val
}

func validPHP(v string) bool {
	for _, s := range app.SupportedPHP {
		if s == v {
			return true
		}
	}
	return false
}
