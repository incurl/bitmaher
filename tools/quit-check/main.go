// A live check of the way the application ends.
//
// The quit of the interface used to be a command that no backend
// answered: the request fell through to the catch-all of the command
// map, was written to the log, and the application stayed alive.
//
// The check starts the real application, asks it to quit the way the
// interface does, and watches two things: that the process ends, and
// that the engine does not outlive it.
//
// The application is run in server mode, because that is the mode in
// which the Wails runtime answers over HTTP. The call is the one the
// frontend makes: the runtime's own quit.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/incurl/bitmaher/internal/aria2"
)

// The runtime of the Wails runtime: the object of the application, and
// the method of the quit.
const (
	runtimeObjectApplication = 2
	runtimeMethodQuit        = 2

	// The call of a bound method: the object of the calls, and the
	// method that binds them.
	runtimeObjectCall = 0
	runtimeMethodCall = 0
)

// appServiceName is the name the runtime addresses the bound methods
// with.
const appServiceName = "github.com/incurl/bitmaher/internal/bindings.AppService"

func main() {
	dir, err := os.MkdirTemp("", "bitmaher-quit")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	binary := applicationBinary()
	fmt.Printf("1. The application is %s\n", binary)

	engine, err := findEngine()
	if err != nil {
		fmt.Println("   FAIL:", err)
		os.Exit(1)
	}
	fmt.Printf("   the engine binary is %s\n", engine)

	// The application runs against an isolated database, so that the
	// check does not touch the data of the user.
	cmd := exec.Command(binary)
	cmd.Env = append(os.Environ(),
		"BITMAHER_DATA_DIR="+dir,
		"BITMAHER_ENGINE_BINARY="+engine,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var output strings.Builder
	cmd.Stdout = &output
	cmd.Stderr = &output

	if err := cmd.Start(); err != nil {
		fmt.Println("   FAIL: start the application:", err)
		os.Exit(1)
	}
	pid := cmd.Process.Pid
	fmt.Printf("2. Started, pid %d\n", pid)

	// The server is given a moment to listen.
	if !waitForServer(20 * time.Second) {
		fmt.Println("   FAIL: the server did not start")
		fmt.Println(output.String())
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		os.Exit(1)
	}
	fmt.Println("3. The server is listening")

	// The engine is a child of the application, and it is the one that
	// holds the port of the RPC endpoint.
	enginePID := waitForEngine(pid, 30*time.Second)
	if enginePID == 0 {
		fmt.Println("   FAIL: the engine did not start")
		fmt.Println(output.String())
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		os.Exit(1)
	}
	fmt.Printf("4. The engine is running, pid %d\n", enginePID)

	// The answers of the features BitMaher does not have are falsy.
	//
	// The runtime answers a method that returns nothing with an empty
	// object, and an empty object is **true** in JavaScript. The
	// frontend read the answer of the question about the autostart as a
	// yes, took the silent branch, and hid the window: it flashed and
	// disappeared at start-up.
	if !theNeutralAnswersAreFalsy() {
		fmt.Println("   FAIL: an answer of a feature BitMaher does not have is true")
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		os.Exit(1)
	}
	fmt.Println("5. The answers of the features BitMaher does not have are falsy")

	// The quit is asked for the way the interface asks for it: the
	// runtime's own quit.
	quitTheApplication()

	// The shutdown runs: the engine is stopped with it.
	//
	// The server build does not leave its process on its own, because
	// the loop of the server waits for a signal. The desktop build
	// leaves as soon as the window is gone. What matters here is the
	// same in both: the quit runs the shutdown, and the shutdown stops
	// the engine.
	if waitForExit(enginePID, 30*time.Second) {
		fmt.Println("6. The engine was stopped by the shutdown")
	} else {
		fmt.Println("   FAIL: the engine outlived the quit")
		_ = syscall.Kill(-enginePID, syscall.SIGKILL)
		os.Exit(1)
	}

	// The application is stopped as well.
	_ = syscall.Kill(-pid, syscall.SIGTERM)
	if !waitForExit(pid, 20*time.Second) {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	}
	fmt.Println("7. The application ended")
	fmt.Println("\nThe quit works: the shutdown runs, and the engine goes with it.")
}

// applicationBinary returns the application the check runs against.
//
// The server build of the repository is used: it is the build in which
// the Wails runtime answers over HTTP, so that the check can make the
// same call the frontend makes.
func applicationBinary() string {
	if path := os.Getenv("BITMAHER_APPLICATION_BINARY"); path != "" {
		return path
	}

	// The check runs from the repository root or from the tools
	// directory, so both places are looked at.
	for _, candidate := range []string{
		filepath.Join("bin", "bitmaher-server"),
		filepath.Join("..", "..", "bin", "bitmaher-server"),
	} {
		path, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	fmt.Println("   FAIL: the application is not built; run wails3 task build:server")
	os.Exit(1)
	return ""
}

// findEngine returns the engine the application runs.
//
// The one on the PATH is used, and the bundled one when there is none.
func findEngine() (string, error) {
	if path := os.Getenv("BITMAHER_ENGINE_BINARY"); path != "" {
		return path, nil
	}

	path, err := aria2.BundledBinaryPath(filepath.Join(os.TempDir(), "bitmaher-check-engine"))
	if err != nil {
		return "/usr/bin/aria2-next", nil
	}
	return path, nil
}

// waitForServer waits for the server to answer.
func waitForServer(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		response, err := http.Get(serverURL + "/wails/runtime.js")
		if err == nil {
			response.Body.Close()
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}

// serverURL is the address of the server of the application.
const serverURL = "http://127.0.0.1:8080"

// quitTheApplication asks the application to end.
//
// The request is the one the frontend's runtime makes: the quit of the
// application.
func quitTheApplication() {
	fmt.Println("   asking the application to quit")

	body := strings.NewReader(fmt.Sprintf(
		`{"object":%d,"method":%d}`, runtimeObjectApplication, runtimeMethodQuit,
	))

	request, err := http.NewRequest(http.MethodPost, serverURL+"/wails/runtime", body)
	if err != nil {
		fmt.Println("   FAIL: the request:", err)
		os.Exit(1)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		fmt.Println("   FAIL: the request:", err)
		os.Exit(1)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		fmt.Println("   FAIL: the request answered", response.Status)
		os.Exit(1)
	}
}

// theNeutralAnswersAreFalsy asks the application for the answers of the
// features BitMaher does not have.
//
// The answers are read through the runtime, the way the frontend reads
// them. A bound method is called by its name, with the name of the
// service in front of it.
func theNeutralAnswersAreFalsy() bool {
	for _, method := range []string{"NeutralBool", "UnsupportedCommand"} {
		args := "[]"
		if method == "UnsupportedCommand" {
			args = `["a_command_that_does_not_exist"]`
		}

		answer, err := callTheBackend(method, args)
		if err != nil {
			fmt.Println("   FAIL: the call of", method+":", err)
			return false
		}
		if answer != "false" {
			fmt.Printf("   FAIL: %s answered %q, want false\n", method, answer)
			return false
		}
	}

	// The value answer has to be nothing, and not an empty object: the
	// frontend reads it as a value it has to work with.
	answer, err := callTheBackend("NeutralValue", "[]")
	if err != nil {
		fmt.Println("   FAIL: the call of NeutralValue:", err)
		return false
	}
	if answer != "null" {
		fmt.Printf("   FAIL: NeutralValue answered %q, want null\n", answer)
		return false
	}

	return true
}

// callTheBackend calls a bound method and returns the raw answer.
func callTheBackend(method, args string) (string, error) {
	body := strings.NewReader(fmt.Sprintf(
		`{"object":%d,"method":%d,"args":{"call-id":"%s","methodName":"%s.%s","args":%s}}`,
		runtimeObjectCall, runtimeMethodCall, method, appServiceName, method, args,
	))

	request, err := http.NewRequest(http.MethodPost, serverURL+"/wails/runtime", body)
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("the request answered %s: %s", response.Status, payload)
	}

	// The answer of the runtime is the value of the call itself.
	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		return "", fmt.Errorf("the answer is not JSON: %w", err)
	}

	switch typed := value.(type) {
	case bool:
		return strconv.FormatBool(typed), nil
	case nil:
		return "null", nil
	default:
		return string(payload), nil
	}
}

// waitForEngine waits for the engine of the application to start.
func waitForEngine(parent int, timeout time.Duration) int {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if pid := childOf(parent); pid != 0 {
			return pid
		}
		time.Sleep(200 * time.Millisecond)
	}
	return 0
}

// childOf returns the first child of a process.
//
// The engine is the only child the application has.
func childOf(parent int) int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}

	for _, entry := range entries {
		pid, err := parsePid(entry.Name())
		if err != nil || pid == parent {
			continue
		}

		if parentOf(pid) == parent {
			return pid
		}
	}
	return 0
}

// parentOf returns the parent of a process.
//
// The field is read from the process's own directory, so that no tool
// outside the system is needed.
func parentOf(pid int) int {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0
	}

	// The command is in brackets, and it may hold spaces: the parent
	// is the field after the closing bracket.
	end := strings.LastIndex(string(data), ")")
	if end < 0 {
		return 0
	}

	fields := strings.Fields(string(data)[end+1:])
	if len(fields) < 2 {
		return 0
	}

	parent, err := parsePid(fields[1])
	if err != nil {
		return 0
	}
	return parent
}

// parsePid reads a pid.
func parsePid(raw string) (int, error) {
	pid := 0
	for _, r := range raw {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("not a pid: %q", raw)
		}
		pid = pid*10 + int(r-'0')
	}
	return pid, nil
}

// waitForExit waits for a process to be gone.
func waitForExit(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if !processExists(pid) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return !processExists(pid)
}

// processExists answers whether a process is still running.
func processExists(pid int) bool {
	_, err := os.Stat(fmt.Sprintf("/proc/%d", pid))
	return err == nil
}
