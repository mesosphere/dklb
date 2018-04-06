package main

import (
	"os"

	"github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/mesosphere/dklb/pkg/version"
)

func main() {
	log := logrus.StandardLogger()
	log.Println("TODO")

	app := kingpin.New("dklb", "DC/OS Kubernetes load-balancer manager").Version(version.Version)
	kingpin.MustParse(app.Parse(os.Args[1:]))
}
