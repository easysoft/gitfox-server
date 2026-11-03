// Copyright 2019 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by the Polyform License
// that can be found in the LICENSE file.

// Package shell provides functions for converting shell commands
// to posix shell scripts.
package shell

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/easysoft/gitfox-community/pkg/runner/extend"
)

// Script converts a slice of individual shell commands to
// a posix-compliant shell script.
func Script(commands []string) string {
	buf := new(bytes.Buffer)
	fmt.Fprintln(buf)
	fmt.Fprintf(buf, optionScript)
	fmt.Fprintf(buf, tmateScript)
	extend.IncludeShell(buf)
	fmt.Fprintln(buf)
	for _, command := range commands {
		// 多行脚本（例如 run 步骤的 script: | 块）如果沿用 echo + 的方式，
		// 整段命令会在执行前一次性写进日志，命令和它的输出就会错行。
		// 这类脚本改用 shell 自带的 xtrace（set -x），由 shell 在每条命令
		// 执行前输出，保证日志顺序与执行顺序一致。
		if isMultiline(command) {
			buf.WriteString(traceMultilineScript)
			buf.WriteString(command)
			if !strings.HasSuffix(command, "\n") {
				buf.WriteString("\n")
			}
			buf.WriteString(endTraceMultilineScript)
			continue
		}

		escaped := fmt.Sprintf("%q", command)
		escaped = strings.Replace(escaped, "$", `\$`, -1)
		buf.WriteString(fmt.Sprintf(
			traceScript,
			escaped,
			command,
		))
	}
	return buf.String()
}

// isMultiline reports whether the command contains internal line breaks.
// A trailing newline does not make a single-line command multi-line.
func isMultiline(command string) bool {
	return strings.Contains(strings.TrimRight(command, " \t\r\n"), "\n")
}

// optionScript is a helper script this is added to the build
// to set shell options, in this case, to exit on error.
const optionScript = `
if [ ! -z "${GITFOX_NETRC_FILE}" ]; then
	echo $GITFOX_NETRC_FILE > $HOME/.netrc
	chmod 600 $HOME/.netrc
fi

unset GITFOX_SCRIPT
unset GITFOX_NETRC_MACHINE
unset GITFOX_NETRC_USERNAME
unset GITFOX_NETRC_PASSWORD
unset GITFOX_NETRC_FILE

set -e
`

// traceScript is a helper script that is added to
// the build script to trace a command.
const traceScript = `
echo + %s
%s
`

// traceMultilineScript and endTraceMultilineScript enable the shell's
// xtrace for a multi-line script so each command is written to the log
// right before it runs. The redirection on `set +x` hides the trace of
// the disable command itself.
const (
	traceMultilineScript    = "set -x\n"
	endTraceMultilineScript = "{ set +x; } 2>/dev/null\n"
)

const tmateScript = `
remote_debug() {
	if [ "$?" -ne "0" ]; then
		/usr/drone/bin/tmate -F
	fi
}

if [ "${GITFOX_BUILD_DEBUG}" = "true" ]; then
	if [ ! -z "${GITFOX_TMATE_HOST}" ]; then
		echo "set -g tmate-server-host $GITFOX_TMATE_HOST" >> $HOME/.tmate.conf
		echo "set -g tmate-server-port $GITFOX_TMATE_PORT" >> $HOME/.tmate.conf
		echo "set -g tmate-server-rsa-fingerprint $GITFOX_TMATE_FINGERPRINT_RSA" >> $HOME/.tmate.conf
		echo "set -g tmate-server-ed25519-fingerprint $GITFOX_TMATE_FINGERPRINT_ED25519" >> $HOME/.tmate.conf

		if [ ! -z "${GITFOX_TMATE_AUTHORIZED_KEYS}" ]; then
			echo "$GITFOX_TMATE_AUTHORIZED_KEYS" > $HOME/.tmate.authorized_keys
			echo "set -g tmate-authorized-keys \"$HOME/.tmate.authorized_keys\"" >> $HOME/.tmate.conf
		fi
	fi
	trap remote_debug EXIT
fi
`
