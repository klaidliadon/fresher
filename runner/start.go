package runner

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"
)

var (
	startChannel chan string
	stopChannel  chan bool
	done         chan bool
	quit         chan os.Signal
	exiting      bool
	mainLog      logFunc
	watcherLog   logFunc
	runnerLog    logFunc
	buildLog     logFunc
	appLog       logFunc
)

func flushEvents() {
	for {
		select {
		case eventName := <-startChannel:
			mainLog("receiving event %s", eventName)
		default:
			return
		}
	}
}

func start() {
	loopIndex := 0
	buildDelay := buildDelay()

	started := false

	go termHandler()

	go func() {
		for {
			loopIndex++
			mainLog("Waiting (loop %d)...", loopIndex)
			eventName := <-startChannel

			mainLog("receiving first event %s", eventName)
			mainLog("sleeping for %d milliseconds", buildDelay)
			time.Sleep(buildDelay * time.Millisecond)
			mainLog("flushing events")

			flushEvents()

			mainLog("Started! (%d Goroutines)", runtime.NumGoroutine())
			err := removeBuildErrorsLog()
			if err != nil {
				mainLog(err.Error())
			}

			buildFailed := false
			if shouldRebuild(eventName) {
				errorMessage, ok := build()
				if !ok {
					buildFailed = true
					mainLog("Build Failed: \n %s", errorMessage)
					if !started {
						os.Exit(1)
					}
					createBuildErrorsLog(errorMessage)
				}
			}

			if !buildFailed {
				if started {
					stopChannel <- true
				}
				run()
			}
			started = true
			mainLog(strings.Repeat("-", 20))
		}
	}()
}

func init() {
	startChannel = make(chan string, 1000)
	stopChannel = make(chan bool)
}

func initLogFuncs() {
	mainLog = newLogFunc("main", true)
	watcherLog = newLogFunc("watcher", true)
	runnerLog = newLogFunc("runner", true)
	buildLog = newLogFunc("build", true)
	appLog = newLogFunc("app", false)
}

func setEnvVars() {
	os.Setenv("DEV_RUNNER", "1")
	wd, err := os.Getwd()
	if err == nil {
		os.Setenv("RUNNER_WD", wd)
	}

	for k, v := range settings {
		key := strings.ToUpper(fmt.Sprintf("%s%s", envSettingsPrefix, k))
		os.Setenv(key, v)
	}
}

func termHandler() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	exiting = true
	stopChannel <- true
}

// Start Watches for file changes in the root directory.
// After each file system event it builds and (re)starts the application.
func Start() {

	done = make(chan bool)

	initLimit()
	initSettings()
	initLogFuncs()
	initFolders()
	setEnvVars()
	watch()
	start()
	startChannel <- "/"

	<-done
}
