/* SPDX-License-Identifier: Apache-2.0
 * Copyright © 2021 VMware, Inc.
 */

package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"syscall"

	"github.com/godbus/dbus/v5"
	"github.com/network-event-broker/pkg/conf"
	"github.com/network-event-broker/pkg/log"
	"github.com/network-event-broker/pkg/network"
	"github.com/network-event-broker/pkg/system"
)

func executeLinkStateScripts(link string, index int, k string, v string) error {
	scriptDirs, err := system.ReadAllScriptDirs(conf.ConfPath)
	if err != nil {
		log.Debugln("Failed to find any scripts in conf dir")
		return err
	}

	for _, d := range scriptDirs {
		stateDir := strings.Trim(v, "\"") + ".d"

		if stateDir == d {
			scripts, err := system.ReadAllScriptInConfDir(path.Join(conf.ConfPath, d))
			if err != nil {
				log.Errorf("Failed to read script dir '%s'", path.Join(conf.ConfPath, d))
				continue
			}

			path.Join(conf.ConfPath, d)
			linkNameEnvArg := "LINK=" + link
			linkIndexEnvArg := "LINKINDEX=" + strconv.Itoa(index)
			linkStateEnvArg := k + "=" + v

			if len(scripts) <= 0 {
				continue
			}

			leaseFile := path.Join("/run/systemd/netif/leases", strconv.Itoa(index))
			leaseLines, err := system.ReadLines(leaseFile)
			if err != nil {
				log.Debugf("Failed to read lease file of link='%v'", link, err)
			}

			var leaseArg string
			if len(leaseLines) > 0 {
				leaseArg = "DHCP_LEASE="
				leaseArg += strings.Join(leaseLines, " ")
			}

			for _, s := range scripts {
				script := path.Join(conf.ConfPath, d, s)

				log.Debugf("Executing script '%s' in dir='%v' for link='%s'", script, d, link)

				cmd := exec.Command(script)
				cmd.Env = append(os.Environ(),
					linkNameEnvArg,
					linkNameEnvArg,
					linkIndexEnvArg,
					linkStateEnvArg,
					leaseArg,
				)

				if err := cmd.Run(); err != nil {
					log.Errorf("Failed to execute script='%s': %v", script, err)
					continue
				}

				log.Debugf("Successfully executed script '%s' in dir='%v' script for link='%s'", script, d, link)
			}
		}
	}

	return nil
}

func executeManagerScripts(k string, v string) error {
	managerStatePath := path.Join(conf.ConfPath, conf.ManagerStateDir)

	scripts, err := system.ReadAllScriptInConfDir(managerStatePath)
	if err != nil {
		log.Errorf("Failed to read script dir '%s'", managerStatePath)
		return nil
	}

	for _, s := range scripts {
		script := path.Join(managerStatePath, s)

		log.Debugf("Executing script '%s' in dir='%v'", script, managerStatePath)

		managerStateEnvArg := k + "=" + v
		cmd := exec.Command(script)
		cmd.Env = append(os.Environ(),
			managerStateEnvArg,
			managerStateEnvArg,
		)

		if err := cmd.Run(); err != nil {
			log.Errorf("Failed to execute script='%s': %v", script, err)
			continue
		}

		log.Debugf("Successfully executed script '%s' in dir='%v' for manager state", script, managerStatePath)
	}

	return nil
}

func processDbusLinkMessage(n *network.Network, v *dbus.Signal) error {
	if !strings.HasPrefix(string(v.Path), "/org/freedesktop/network1/link/_3") {
		return nil
	}

	strIndex := strings.TrimPrefix(string(v.Path), "/org/freedesktop/network1/link/_3")
	index, err := strconv.Atoi(strIndex)
	if err != nil {
		log.Errorf("Failed to convert ifindex to integer: %v", strIndex)
		return nil
	}

	log.Debugf("Received DBus signal from systemd-networkd for ifindex='%d' link='%s'", index, n.LinksByIndex[index])

	_, ok := n.LinksByName[n.LinksByIndex[2]]
	if !ok {
		log.Debugf("Link='%d' ifindex='%d' not configured in configuration. Ignoring", index, n.LinksByIndex[index])
		return nil
	}

	linkState := v.Body[1].(map[string]dbus.Variant)
	for k, v := range linkState {
		switch k {
		case "OperationalState":
			{
				log.Infof("Link='%v' ifindex='%v' changed state '%s'=%s", n.LinksByIndex[index], index, k, v)

				executeLinkStateScripts(n.LinksByIndex[index], index, k, v.String())
			}
		}
	}

	return nil
}

func processDbusManagerMessage(n *network.Network, v *dbus.Signal) error {
	state := v.Body[1].(map[string]dbus.Variant)

	for k, v := range state {
		log.Debugf("Manager chaged state '%v='%v'", k, v.String())
		executeManagerScripts(k, v.String())
	}

	return nil
}

func main() {
	err := log.Init("info")
	if err != nil {
		log.Warnf("Failed to configure logging: %v", err)
		os.Exit(1)
	}

	_, err = conf.Parse()
	if err != nil {
		log.Fatalf("Failed to parse configuration: %v", err)
		os.Exit(1)
	}

	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		log.Fatalf("Failed to connect to system bus: %v", err)
		os.Exit(1)
	}
	defer conn.Close()

	opts := []dbus.MatchOption{
		dbus.WithMatchSender("org.freedesktop.network1"),
		dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		dbus.WithMatchMember("PropertiesChanged"),
	}

	if err := conn.AddMatchSignal(opts...); err != nil {
		log.Errorf("Failed to add match signal for 'org.freedesktop.network1`: %v", err)
		os.Exit(1)
	}

	c := make(chan *dbus.Signal, 512)
	conn.Signal(c)

	for v := range c {

		/* Refresh link information */
		n, err := network.AcquireLinks()
		if err != nil {
			log.Fatalf("Failed to acquire link information. Unable to continue: %v", err)
			os.Exit(1)
		}

		w := fmt.Sprintf("%v", v.Body[0])

		if strings.HasPrefix(w, "org.freedesktop.network1.Link") {
			log.Debugf("Received Link DBus signal from systemd-networkd'")
			go processDbusLinkMessage(n, v)
		} else if strings.HasPrefix(w, "org.freedesktop.network1.Manager") {
			log.Debugf("Received Manager DBus signal from systemd-networkd'")
			go processDbusManagerMessage(n, v)
		}
	}

	s := make(chan os.Signal, 1)
	signal.Notify(s, os.Interrupt)
	signal.Notify(s, syscall.SIGTERM)
	go func() {
		<-s
		os.Exit(0)
	}()
}