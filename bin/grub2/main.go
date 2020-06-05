/*
 * Copyright (C) 2017 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     jouyouyun <jouyouwen717@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"bufio"
	"flag"
	"io"
	"os"
	"strings"

	"os/exec"

	"pkg.deepin.io/dde/daemon/grub2"
	"pkg.deepin.io/lib/log"
)

var logger = log.NewLogger("daemon/grub2")

var (
	optPrepareGfxmodeDetect bool
	optFinishGfxmodeDetect  bool
	optSetupTheme           bool
	optDebug                bool
)

func init() {
	grub2.SetLogger(logger)

	flag.BoolVar(&optDebug, "debug", false, "debug mode")
	flag.BoolVar(&optPrepareGfxmodeDetect, "prepare-gfxmode-detect", false,
		"prepare gfxmode detect")
	flag.BoolVar(&optFinishGfxmodeDetect, "finish-gfxmode-detect", false,
		"finish gfxmode detect")
	flag.BoolVar(&optSetupTheme, "setup-theme", false, "do nothing")
}

func main() {
	flag.Parse()
	if optDebug {
		logger.SetLogLevel(log.LevelDebug)
	}

	// fix os /boot firm in huawei
	bootMount, err := ReadBootLine("/proc/self/mounts")
	if err != nil {
		logger.Warning("load bootMount error")
	}
	if strings.Contains(bootMount[0], "ro") {
		outs, err := exec.Command("/bin/sh", "-c", "mount -o rw,remount /boot").CombinedOutput()
		if err != nil {
			logger.Warning("Failed to remount /boot to rw:", string(outs), err)
			os.Exit(2)
		}
		defer func() {
			outs, err := exec.Command("/bin/sh", "-c", "mount -o ro,remount /boot").CombinedOutput()
			if err != nil {
				logger.Warning("Failed to remount /boot to ro:", string(outs), err)
			}
		}()
	}

	if optPrepareGfxmodeDetect {
		logger.Debug("mode: prepare gfxmode detect")
		err := grub2.PrepareGfxmodeDetect()
		if err != nil {
			logger.Warning(err)
			os.Exit(2)
		}
	} else if optFinishGfxmodeDetect {
		logger.Debug("mode: finish gfxmode detect")
		err := grub2.FinishGfxmodeDetect()
		if err != nil {
			logger.Warning(err)
			os.Exit(2)
		}
	} else if optSetupTheme {
		// for compatibility
		return
	} else {
		logger.Debug("mode: daemon")
		grub2.RunAsDaemon()
	}
}

func ReadBootLine(fileName string) ([]string, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	buf := bufio.NewReader(f)
	var result []string
	for {
		line, err := buf.ReadString('\n')
		line = strings.TrimSpace(line)
		if err != nil {
			if err == io.EOF { //读取结束，会报EOF
				return result, nil
			}
			return nil, err
		}
		if strings.Contains(line, " /boot ") {
			result = append(result, line)
		}
	}
	return result, nil
}
