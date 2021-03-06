// psn inspects Linux processes, sockets (ps, ss, netstat).
//
//	Usage:
//	psn [command]
//
//	Available Commands:
//	ds          Inspects '/proc/diskstats'
//	ns          Inspects '/proc/net/dev'
//	ps          Inspects '/proc/$PID/stat,status'
//	ss          Inspects '/proc/net/tcp,tcp6'
//
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	command = &cobra.Command{
		Use:        "psn",
		Short:      "psn inspects Linux processes, sockets (ps, ss, netstat).",
		SuggestFor: []string{"pssn", "psns", "snp"},
	}
)

func init() {
	command.AddCommand(dsCommand)
	command.AddCommand(nsCommand)
	command.AddCommand(psCommand)
	command.AddCommand(ssCommand)
}

func init() {
	cobra.EnablePrefixMatching = true
}

func main() {
	if err := command.Execute(); err != nil {
		fmt.Fprintln(os.Stdout, err)
		os.Exit(1)
	}
}
