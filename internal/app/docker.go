package app

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// ── Docker primitives ────────────────────────────────────────────────────────

func DockerRun(args ...string) (string, error) {
	cmd := exec.Command("docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("%w: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

func DockerCompose(file string, args ...string) (string, error) {
	return DockerRun(append([]string{"compose", "-f", file}, args...)...)
}

// ── Checks ───────────────────────────────────────────────────────────────────

func CheckDocker() error {
	if _, err := DockerRun("info"); err != nil {
		return fmt.Errorf("docker not running: %w", err)
	}
	return nil
}

func NetworkExists() bool {
	out, _ := DockerRun("network", "ls", "--filter", "name=^"+NetworkName+"$", "--format", "{{.Name}}")
	return strings.TrimSpace(out) == NetworkName
}

func ContainerRunning(name string) bool {
	out, _ := DockerRun("inspect", "-f", "{{.State.Running}}", name)
	return strings.TrimSpace(out) == "true"
}

func ContainerExists(name string) bool {
	_, err := DockerRun("inspect", name)
	return err == nil
}

// ── Infrastructure ───────────────────────────────────────────────────────────

func CreateNetwork() error {
	_, err := DockerRun("network", "create", NetworkName)
	if err != nil && strings.Contains(err.Error(), "already exists") {
		return nil
	}
	return err
}

func EnsureInfra(c *Config) error {
	if !NetworkExists() {
		if err := CreateNetwork(); err != nil {
			return fmt.Errorf("network: %w", err)
		}
	}
	if err := StartMySQL(c); err != nil {
		return fmt.Errorf("mysql: %w", err)
	}
	if err := StartRedis(c); err != nil {
		return fmt.Errorf("redis: %w", err)
	}
	return nil
}

func StartMySQL(c *Config) error {
	if ContainerRunning(MySQLContainer) {
		return nil
	}
	if ContainerExists(MySQLContainer) {
		_, err := DockerRun("start", MySQLContainer)
		return err
	}
	_, err := DockerRun("run", "-d",
		"--name", MySQLContainer, "--network", NetworkName, "--restart", "unless-stopped",
		"-p", c.MySQLPort+":3306",
		"-e", "MYSQL_ROOT_PASSWORD="+c.MySQLRootPass,
		"-e", "MYSQL_USER="+c.MySQLUser,
		"-e", "MYSQL_PASSWORD="+c.MySQLPass,
		"-v", "frankenphp-mysql-data:/var/lib/mysql",
		MySQLImage,
	)
	return err
}

func StartRedis(c *Config) error {
	if ContainerRunning(RedisContainer) {
		return nil
	}
	if ContainerExists(RedisContainer) {
		_, err := DockerRun("start", RedisContainer)
		return err
	}
	_, err := DockerRun("run", "-d",
		"--name", RedisContainer, "--network", NetworkName, "--restart", "unless-stopped",
		"-p", c.RedisPort+":6379",
		"-v", "frankenphp-redis-data:/data",
		RedisImage,
	)
	return err
}

func ExecMySQL(c *Config, db, sql string) (string, error) {
	args := []string{"exec", MySQLContainer, "mysql", "-u", "root", "-p" + c.MySQLRootPass}
	if db != "" {
		args = append(args, db)
	}
	args = append(args, "-e", sql)
	return DockerRun(args...)
}

// ── Project containers ───────────────────────────────────────────────────────

func ContainerName(project string) string { return "frankenphp-" + project }

func ProjectStart(name string) error {
	_, err := DockerCompose(ComposePath(name), "up", "-d")
	return err
}

func ProjectStop(name string) error {
	_, err := DockerCompose(ComposePath(name), "down")
	return err
}

func ProjectRunning(name string) bool {
	return ContainerRunning(ContainerName(name))
}

func ProjectLogs(name string, follow bool, tail string) *exec.Cmd {
	args := []string{"logs"}
	if follow {
		args = append(args, "-f")
	}
	if tail != "" {
		args = append(args, "--tail", tail)
	}
	args = append(args, ContainerName(name))
	return exec.Command("docker", args...)
}

func GlobalLogs(follow bool, tail string) (*exec.Cmd, error) {
	out, err := DockerRun("ps", "--filter", "name=frankenphp-", "--format", "{{.Names}}")
	if err != nil {
		return nil, err
	}
	containers := strings.Fields(strings.TrimSpace(out))
	if len(containers) == 0 {
		return nil, fmt.Errorf("no running containers")
	}
	args := []string{"logs"}
	if follow {
		args = append(args, "-f")
	}
	if tail != "" {
		args = append(args, "--tail", tail)
	}
	args = append(args, containers...)
	return exec.Command("docker", args...), nil
}

func RemoveContainer(name string)          { DockerRun("rm", "-f", name) }
func RemoveNetwork() error                 { _, err := DockerRun("network", "rm", NetworkName); return err }
func RemoveVolume(name string) error       { _, err := DockerRun("volume", "rm", "-f", name); return err }
func ListFrankenphpContainers() []string {
	out, _ := DockerRun("ps", "-a", "--filter", "name=frankenphp", "--format", "{{.Names}}")
	return strings.Fields(strings.TrimSpace(out))
}
