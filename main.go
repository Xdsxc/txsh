package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/xdsxc/txsh/internal/config"
	"github.com/xdsxc/txsh/internal/shell"
	"github.com/xdsxc/txsh/internal/twilio"
)

var manager *shell.SessionManager

func SMSCallBack(id, body string) string {
	logger := logrus.WithFields(logrus.Fields{
		"component": "SMSCallBack",
		"caller_id": id,
	})

	var retMsg string
	if strings.HasPrefix(body, "txsh") {
		retMsg = AdminMenuCallback(id, body)
	} else {
		session, err := manager.GetSession(id)
		if err != nil {
			logger.WithError(err).WithField("cmd", body).Errorf("error getting session")
			return err.Error()
		}

		logger.Debugln(body)
		retMsg, err = session.Do(body)
		if err != nil {
			logger.WithError(err).WithField("cmd", body).Errorf("error executing cmd")
			return err.Error()
		}
	}

	// SMS's look weird if there is a null byte or no content
	if len(retMsg) == 0 {
		return "<no content>"
	}

	return retMsg
}

func AdminMenuCallback(id, body string) string {
	components := strings.Split(body, " ")
	if len(components) < 2 {
		return "unknown cmd"
	}

	logger := logrus.WithFields(logrus.Fields{
		"component": "AdminMenuCallback",
		"caller_id": id,
		"cmd":       body,
	})

	switch components[1] {
	case "reset", "stop":
		if err := manager.StopSessionByID(id); err != nil {
			logger.WithError(err).Errorf("error closing session")
		}
		return "completed"
	default:
		logger.Infof("unknown cmd")
		break
	}

	return "unknown cmd"
}

func cleanup() {
	if err := manager.StopAll(); err != nil {
		logrus.WithError(err).Errorf("error cleaning up")
	}
}

func trapCleanup() {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-c
		logrus.Infoln("caught signal; starting cleanup")
		cleanup()
		os.Exit(0)
	}()
}

func main() {
	trapCleanup()
	logger := logrus.WithField("component", "main")
	cfg := config.Config{}
	if err := cfg.Parse(); err != nil {
		logger.WithError(err).Fatalf("error parsing config")
	}

	handler := twilio.NewSMSHandler(SMSCallBack)

	manager = shell.NewSessionManager()
	http.Handle("/", &handler)
	logger.Infof("listening on port %d", cfg.Twilio.Receiver.Port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", cfg.Twilio.Receiver.Port), nil); err != nil {
		logger.WithError(err).Fatalf("main: error starting up http server")
	}
}
