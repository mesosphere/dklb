/*
 * Copyright (c) 2018 Mesosphere, Inc
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
