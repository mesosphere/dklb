package main

import (
	"os"

	"github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/mesosphere/dklb/pkg/version"
)

func main() {
	log := logrus.StandardLogger()

	args := os.Args[1:]
	app := kingpin.New("dklb", "DC/OS Kubernetes load-balancer manager").Version(version.Version)
	if len(args) == 0 {
		app.Usage(args)
		os.Exit(2)
	}

	kingpin.MustParse(app.Parse(args))

	log.Println("TODO")
}
