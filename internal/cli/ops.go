package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"frankenphp-manager/internal/app"
)

// ── log ──────────────────────────────────────────────────────────────────────

var logCmd = &cobra.Command{
	Use:   "log [project|all]",
	Short: "View logs (-f to follow, -t for tail lines)",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		follow, _ := cmd.Flags().GetBool("follow")
		tail, _ := cmd.Flags().GetString("tail")
		name := "all"
		if len(args) > 0 {
			name = args[0]
		}

		var c *exec.Cmd
		var err error
		if name == "all" {
			c, err = app.GlobalLogs(follow, tail)
		} else {
			c, err = app.ProjectLogs(name, follow, tail), nil
		}
		if err != nil {
			die("%v", err)
		}

		c.Stdout, c.Stderr = os.Stdout, os.Stderr
		if follow {
			color.Cyan("Following '%s' (Ctrl+C to stop)...\n", name)
			sig := make(chan os.Signal, 1)
			signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
			go func() { <-sig; fmt.Println(); c.Process.Signal(syscall.SIGTERM) }()
		}
		if err := c.Run(); err != nil && !follow {
			die("%v", err)
		}
	},
}

func init() {
	logCmd.Flags().BoolP("follow", "f", false, "Follow in real-time")
	logCmd.Flags().StringP("tail", "t", "100", "Lines from end")
}

// ── manage ───────────────────────────────────────────────────────────────────

var manageCmd = &cobra.Command{Use: "manage", Short: "Database & cache management"}

var manageCreatedbCmd = &cobra.Command{
	Use:   "createdb [project]", Short: "Create database for project", Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		c, _ := app.LoadConfig()
		p, found := c.GetProject(args[0])
		if !found {
			die("Project not found")
		}
		if p.DBName == "" {
			die("No database configured for '%s'", args[0])
		}
		_, err := app.ExecMySQL(c, "", fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", p.DBName))
		must(err)
		ok("Database '%s' created", p.DBName)
	},
}

var manageDropdbCmd = &cobra.Command{
	Use:   "dropdb [project]", Short: "Drop database for project", Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		c, _ := app.LoadConfig()
		p, found := c.GetProject(args[0])
		if !found {
			die("Project not found")
		}
		if p.DBName == "" {
			die("No database configured")
		}
		_, err := app.ExecMySQL(c, "", fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", p.DBName))
		must(err)
		ok("Database '%s' dropped", p.DBName)
	},
}

var manageListdbCmd = &cobra.Command{
	Use: "listdb", Short: "List all databases",
	Run: func(cmd *cobra.Command, args []string) {
		c, _ := app.LoadConfig()
		out, err := app.ExecMySQL(c, "", "SHOW DATABASES")
		must(err)
		fmt.Println(out)
	},
}

var manageQueryCmd = &cobra.Command{
	Use:   "query [database] [sql]",
	Short: "Execute SQL query",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		c, _ := app.LoadConfig()
		out, err := app.ExecMySQL(c, args[0], args[1])
		must(err)
		fmt.Println(out)
	},
}

var manageMySQLCmd = &cobra.Command{
	Use: "mysql", Short: "Print MySQL connection info",
	Run: func(cmd *cobra.Command, args []string) {
		c, _ := app.LoadConfig()
		fmt.Printf("  docker exec -it %s mysql -u root -p%s\n", app.MySQLContainer, c.MySQLRootPass)
		fmt.Printf("  # host: mysql -h 127.0.0.1 -P %s -u root -p%s\n", c.MySQLPort, c.MySQLRootPass)
	},
}

var manageRedisCmd = &cobra.Command{
	Use: "redis", Short: "Print Redis connection info",
	Run: func(cmd *cobra.Command, args []string) {
		c, _ := app.LoadConfig()
		fmt.Printf("  docker exec -it %s redis-cli\n", app.RedisContainer)
		fmt.Printf("  # host: redis-cli -p %s\n", c.RedisPort)
	},
}

func init() {
	manageCmd.AddCommand(manageCreatedbCmd, manageDropdbCmd, manageListdbCmd, manageQueryCmd, manageMySQLCmd, manageRedisCmd)
}

// ── use ──────────────────────────────────────────────────────────────────────

var useCmd = &cobra.Command{
	Use:   "use [project] [php-version]",
	Short: "Switch PHP version (8.1, 8.2, 8.3, 8.4)",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		c, _ := app.LoadConfig()
		name, newVer := args[0], args[1]
		if !validPHP(newVer) {
			die("Unsupported: %s. Use: %v", newVer, app.SupportedPHP)
		}
		p, exists := c.GetProject(name)
		if !exists {
			die("Project '%s' not found", name)
		}
		if p.PHPVer == newVer {
			info("Already on PHP %s", newVer)
			return
		}
		wasRunning := app.ProjectRunning(name)
		if wasRunning {
			info("Stopping %s...", name)
			app.ProjectStop(name)
		}
		old := p.PHPVer
		p.PHPVer = newVer
		must(app.GenerateProject(p, c))
		must(c.Save())
		if wasRunning {
			info("Starting with PHP %s...", newVer)
			must(app.ProjectStart(name))
		}
		ok("%s: PHP %s → %s", name, old, newVer)
	},
}

// ── tls ──────────────────────────────────────────────────────────────────────

var tlsCmd = &cobra.Command{
	Use:   "tls [project] [http|selfsigned|acme]",
	Short: "Switch HTTPS mode for a project",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		c, _ := app.LoadConfig()
		name, mode := args[0], args[1]
		if mode != "http" && mode != "selfsigned" && mode != "acme" {
			die("Invalid mode: %s. Use: http, selfsigned, acme", mode)
		}
		p, exists := c.GetProject(name)
		if !exists {
			die("Project '%s' not found", name)
		}

		if mode == "acme" {
			r := bufio.NewReader(os.Stdin)
			if p.Domain == "" {
				p.Domain = prompt(r, "  Domain (e.g. example.com)", "")
				if p.Domain == "" {
					die("Domain required for ACME")
				}
			}
			if p.ACMEEmail == "" {
				p.ACMEEmail = prompt(r, "  ACME email", "")
				if p.ACMEEmail == "" {
					die("Email required for ACME")
				}
			}
			// ACME needs public interface
			if p.Iface == "lo" {
				defIface := app.DefaultInterface()
				info("ACME needs a public interface (current: %s)", p.Iface)
				newIface := prompt(r, "  Interface", defIface)
				if newIface != "lo" {
					ipv4, ipv6, err := app.ResolveInterface(newIface)
					if err != nil {
						die("Interface error: %v", err)
					}
					p.Iface = newIface
					p.BindIPv4 = ipv4
					p.BindIPv6 = ipv6
					info("Bind updated: IPv4=%s IPv6=%s", ipv4, ipv6)
				}
			}
		}

		old := p.HTTPMode
		p.HTTPMode = mode

		wasRunning := app.ProjectRunning(name)
		if wasRunning {
			info("Stopping %s...", name)
			app.ProjectStop(name)
		}
		must(app.GenerateProject(p, c))
		must(c.Save())
		if wasRunning {
			info("Starting %s...", name)
			must(app.ProjectStart(name))
		}
		ok("%s: %s → %s (iface %s, bind %s)", name, old, mode, p.Iface, p.BindIPv4)
	},
}

func init() {
	rootCmd.AddCommand(tlsCmd)
}

// ── status ───────────────────────────────────────────────────────────────────

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show system status",
	Run: func(cmd *cobra.Command, args []string) {
		c, _ := app.LoadConfig()

		color.Cyan("╔══════════════════════════════════════╗")
		color.Cyan("║     FrankenPHP Manager — Status      ║")
		color.Cyan("╚══════════════════════════════════════╝\n")

		dockerOK := color.GreenString("running")
		if app.CheckDocker() != nil {
			dockerOK = color.RedString("not running")
		}
		fmt.Printf("  Docker:   %s\n", dockerOK)

		netOK := color.RedString("missing")
		if app.NetworkExists() {
			netOK = color.GreenString("ok")
		}
		fmt.Printf("  Network:  %s (%s)\n", netOK, app.NetworkName)

		mysqlSt := color.RedString("stopped")
		if app.ContainerRunning(app.MySQLContainer) {
			mysqlSt = color.GreenString("running")
		}
		fmt.Printf("  MySQL:    %s (port %s)\n", mysqlSt, c.MySQLPort)

		redisSt := color.RedString("stopped")
		if app.ContainerRunning(app.RedisContainer) {
			redisSt = color.GreenString("running")
		}
		fmt.Printf("  Redis:    %s (port %s)\n", redisSt, c.RedisPort)

		projects := c.ProjectList()
		fmt.Printf("  Projects: %d\n", len(projects))

		if len(projects) > 0 {
			fmt.Println()
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "  PROJECT\tIFACE\tBIND\tPORT\tSTATUS\tHTTPS\n")
			fmt.Fprintf(w, "  ───────\t─────\t────\t────\t──────\t─────\n")
			for _, p := range projects {
				st := color.RedString("stopped")
				if app.ContainerRunning(app.ContainerName(p.Name)) {
					st = color.GreenString("running")
				}
				https := "http"
				if p.HTTPMode == "acme" {
					https = "acme → " + p.Domain
				} else if p.HTTPMode == "selfsigned" {
					https = "self-signed"
				}
				iface := orDefault(p.Iface, "lo")
				bind := orDefault(p.BindIPv4, "—")
				fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\t%s\n",
					p.Name, iface, bind, p.Port, st, https)
			}
			w.Flush()
		}

		// Show interface summary
		fmt.Println()
		color.Cyan("  Interfaces:")
		ifaces, _ := app.ListInterfaces()
		for _, name := range ifaces {
			fmt.Printf("    • %s\n", app.DescribeInterface(strings.TrimSuffix(name, " (loopback)")))
		}

		fmt.Printf("\n  Config: %s\n", app.ConfigPath())
	},
}

// ── destroy ──────────────────────────────────────────────────────────────────

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Remove ALL containers, volumes, networks, config",
	Run: func(cmd *cobra.Command, args []string) {
		force, _ := cmd.Flags().GetBool("force")
		if !force {
			color.Red("⚠ This deletes EVERYTHING:")
			fmt.Println("  • All FrankenPHP containers")
			fmt.Println("  • MySQL + all databases")
			fmt.Println("  • Redis + all data")
			fmt.Println("  • All config files\n")
			fmt.Print("  Type 'yes' to confirm: ")
			in, _ := bufio.NewReader(os.Stdin).ReadString('\n')
			if strings.TrimSpace(in) != "yes" {
				fmt.Println("  Cancelled.")
				return
			}
		}

		c, _ := app.LoadConfig()
		info("Stopping projects...")
		for _, p := range c.Projects {
			app.ProjectStop(p.Name)
		}
		info("Stopping MySQL & Redis...")
		app.DockerRun("stop", app.MySQLContainer)
		app.DockerRun("stop", app.RedisContainer)
		info("Removing containers...")
		for _, name := range app.ListFrankenphpContainers() {
			app.RemoveContainer(name)
		}
		info("Removing volumes...")
		app.RemoveVolume("frankenphp-mysql-data")
		app.RemoveVolume("frankenphp-redis-data")
		info("Removing network...")
		app.RemoveNetwork()
		info("Removing config...")
		os.RemoveAll(app.ConfigDir())
		ok("All FrankenPHP resources destroyed")
	},
}

func init() {
	destroyCmd.Flags().BoolP("force", "f", false, "Skip confirmation")
}
