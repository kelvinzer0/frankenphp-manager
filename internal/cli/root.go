package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"frankenphp-manager/internal/app"
)

var appVersion string

func SetVersion(v string) { appVersion = v }

var rootCmd = &cobra.Command{
	Use:     "frankenphp",
	Short:   "FrankenPHP Manager — CLI PHP server manager",
	Long:    "Manage PHP dev servers with FrankenPHP, MySQL, Redis via Docker.",
	Version: appVersion,
}

func Execute() error { return rootCmd.Execute() }

func init() {
	rootCmd.AddCommand(initCmd, addCmd, startCmd, stopCmd, restartCmd,
		listCmd, logCmd, useCmd, manageCmd, removeCmd, statusCmd, destroyCmd)
}

// ── init ─────────────────────────────────────────────────────────────────────

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "First-time setup: Docker check, network, MySQL, Redis",
	Run: func(cmd *cobra.Command, args []string) {
		color.Cyan("╔══════════════════════════════════════╗")
		color.Cyan("║     FrankenPHP Manager — Setup       ║")
		color.Cyan("╚══════════════════════════════════════╝\n")

		info("Checking Docker...")
		if err := app.CheckDocker(); err != nil {
			die("Docker not running. Install: https://docs.docker.com/get-docker/")
		}
		ok("Docker is running")

		c, _ := app.LoadConfig()
		r := bufio.NewReader(os.Stdin)

		if c.MySQLRootPass == "frankenphp" {
			color.Yellow("Database config (Enter = default):")
			c.MySQLRootPass = prompt(r, "  MySQL root password", c.MySQLRootPass)
			c.MySQLUser = prompt(r, "  MySQL user", c.MySQLUser)
			c.MySQLPass = prompt(r, "  MySQL password", c.MySQLPass)
			c.MySQLPort = prompt(r, "  MySQL port", c.MySQLPort)
			c.RedisPort = prompt(r, "  Redis port", c.RedisPort)
		}

		info("Creating network '%s'...", app.NetworkName)
		must(app.CreateNetwork())
		info("Starting MySQL...")
		must(app.StartMySQL(c))
		ok("MySQL on port %s", c.MySQLPort)
		info("Starting Redis...")
		must(app.StartRedis(c))
		ok("Redis on port %s", c.RedisPort)
		must(c.Save())

		fmt.Println()
		color.Green("✔ Setup complete! Next: frankenphp add /path/to/project")
	},
}

// ── helpers ──────────────────────────────────────────────────────────────────

func die(msg string, a ...any)   { fmt.Fprintf(os.Stderr, "✖ "+msg+"\n", a...); os.Exit(1) }
func ok(msg string, a ...any)    { fmt.Printf("✔ "+msg+"\n", a...) }
func info(msg string, a ...any)  { fmt.Printf("  → "+msg+"\n", a...) }
func warn(msg string, a ...any)  { fmt.Printf("⚠ "+msg+"\n", a...) }
func must(err error)             { if err != nil { die("%v", err) } }

func prompt(r *bufio.Reader, label, def string) string {
	fmt.Printf("%s [%s]: ", label, def)
	in, _ := r.ReadString('\n')
	in = strings.TrimSpace(in)
	if in == "" {
		return def
	}
	return in
}
