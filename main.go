package main

import (
	"fmt"
	"os"
	"time"

	"os/exec"
	"strconv"
	"strings"

	"flag"

	"path/filepath"

	"log"

	"barista.run"
	"barista.run/bar"
	"barista.run/base/click"
	"barista.run/colors"
	"barista.run/format"
	"barista.run/modules/battery"
	"barista.run/modules/clock"
	"barista.run/modules/diskspace"
	"barista.run/modules/netinfo"
	"barista.run/modules/shell"
	"barista.run/modules/wlan"
	"barista.run/outputs"
	"os/user"
)

const (
	Program = "my-i3statusbar"

	padding = 25
)

// RunRight executes the given command on a left-click. This is a shortcut for
// click.Right(func(){exec.Command(cmd).Run()}).
func RunRight(cmd string, args ...string) func(bar.Event) {
	return click.Right(func() {
		log.Printf("Executing %s %s on right click", cmd, strings.Join(args, " "))
		exec.Command(cmd, args...).Run()
	})
}

func RunLeft(cmd string, args ...string) func(bar.Event) {
	return click.Left(func() {
		log.Printf("Executing %s %s on left click", cmd, strings.Join(args, " "))
		exec.Command(cmd, args...).Run()
	})
}

func sudo(cmd string, args... string) error {
	sudo := "sudo"
	log.Printf("Executing %s %s %s", sudo, cmd, strings.Join(args, " "))
	args = append([]string{cmd}, args...)
	out, err := exec.Command(sudo, args...).CombinedOutput()
	log.Printf("Output of '%s %s %s: %s", sudo, cmd, strings.Join(args, " "), string(out))
	if err != nil {
		return fmt.Errorf("executing %s %s %s: %v", sudo, cmd, strings.Join(args, " "), err)
	}
	return nil
}

func dhclient(args... string) error {
	dhclient := "/usr/sbin/dhclient"
	err := sudo(dhclient, args...)
	if err != nil {
		return fmt.Errorf("could not run %s: %v", dhclient, err)
	}
	return nil
}

func wpaSupplicant(config, iface string) error {
	wpaSupplicant := "/usr/sbin/wpa_supplicant"
	err := sudo(wpaSupplicant, "-c", config, "-i", iface, "-B")
	if err != nil {
		return fmt.Errorf("could not run %s: %v", wpaSupplicant, err)
	}
	return nil
}

func pkill(pattern string) error {
	pkill := "/usr/bin/pkill"
	_ = sudo(pkill, pattern)
	return nil
}


func startWifi(config, iface string) error {
	log.Printf("Starting wifi...")
	if err := wpaSupplicant(config, iface); err != nil {
		log.Printf("Error running wpa_supplicant: %v", err)
		return fmt.Errorf("Wifi not started")
	}

	if err := dhclient(); err != nil {
		log.Printf("Error running dhclient: %v", err)
		return fmt.Errorf("Wifi not started")
	}
	log.Printf("Wifi started")
	return nil
}

func stopWifi() error {
	log.Printf("Stopping wifi...")
	if err := dhclient("-r"); err != nil {
		log.Printf("Error running dhclient -r: %v", err)
		return fmt.Errorf("Wifi not stopped")
	}

	if err := pkill("wpa_supplicant"); err != nil {
		log.Printf("Error running pkill wpa_supplicant: %v", err)
		return fmt.Errorf("Wifi not stopped")
	}
	log.Printf("Stopped wifi")
	return nil
}

func restartWifi(config, iface string) {
	log.Printf("Restarting wifi...")
	if err := stopWifi(); err != nil {
		log.Printf("Wifi not restarted")
		return
	}
	if err := startWifi(config, iface); err != nil {
		log.Printf("Wifi not restarted")
		return
	}
	log.Printf("Wifi restarted")
}

func main() {
	logDir := filepath.Join(os.Getenv("HOME"), ".local", "share", Program)
	if fileInfo, err := os.Stat(logDir); os.IsNotExist(err) {
		log.Printf("Creating %s as it does not exist", logDir)
		os.Mkdir(logDir, 0777)
	} else if !fileInfo.IsDir() {
		log.Fatalf("%s exists but it is not a directory", logDir)
	}
	logFile := filepath.Join(logDir, fmt.Sprintf("%s_%s.log", Program, time.Now().Format("20060102")))
	f, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0777)
	if err != nil {
		log.Fatalf("Error: Could not open %s: %v", logFile, err)
	}
	defer f.Close()

	log.SetOutput(f)

	log.Printf(`
================================================================================
	                           Started %s", Program
================================================================================
`, Program)
	defer log.Printf(`
================================================================================
	                           Stopped %s", Program
================================================================================
`, Program)

	if usr, err := user.Current(); err == nil {
		log.Printf("Running %s as %s (%s)", Program, usr.Username, usr.Name)
	} else {
		log.Printf("Error: Could not get the current user: %v", err)
	}

	email := flag.String("email", "", "Email for LastPass account")
	config := flag.String("config", "", "Path to the wpa_supplicant config")
	iface := flag.String("iface", "", "Interface to connect to wifi via")
	flag.Parse()

	colors.LoadFromMap(map[string]string{
		"good":     "#0f0",
		"bad":      "#f00",
		"degraded": "#ff0",
	})

	if *email != "" {
		log.Printf("Email given, enabling lastpass status")
		barista.Add(shell.New("bash", "-c", "lpass status || exit 0").Output(func(output string) bar.Output {
			out := outputs.Text("LastPass").Padding(padding)
			if strings.Contains(output, "Logged in") {
				return out.Color(colors.Scheme("good"))
			}
			return out.Color(colors.Scheme("bad")).
				OnClick(RunLeft("lpass", "login", *email))
		}).Every(time.Second))
	}

	// Show the amount of available disk space in the root partition
	barista.Add(diskspace.New("/").Output(func(i diskspace.Info) bar.Output {
		out := outputs.Textf("/: %s", format.IBytesize(i.Available))
		switch {
		case i.AvailFrac() < 0.2:
			out.Color(colors.Scheme("bad"))
		case i.AvailFrac() < 0.33:
			out.Color(colors.Scheme("degraded"))
		}
		return out.Padding(padding)
	}))

	barista.Add(diskspace.New("/home").Output(func(i diskspace.Info) bar.Output {
		out := outputs.Textf("/home: %s", format.IBytesize(i.Available))
		switch {
		case i.AvailFrac() < 0.2:
			out.Color(colors.Scheme("bad"))
		case i.AvailFrac() < 0.33:
			out.Color(colors.Scheme("degraded"))
		}
		return out.Padding(padding)
	}))

	// Show WiFi with
	barista.Add(wlan.Any().Output(func(w wlan.Info) bar.Output {
		switch {
		case w.Connected():
			shellCmd := fmt.Sprintf(`iwconfig %s | egrep -o 'Link Quality=[0-9]+\/[0-9]+' | awk -F '=' '{ print $2 }'`, w.Name)
			cmd := "bash"
			args := []string{"-c", shellCmd}
			stdout, err := exec.Command(cmd, args...).Output()
			if err != nil {
				log.Printf("Error running %s %s: %v", cmd, strings.Join(args, " "), err)
				return nil
			}
			parts := strings.Split(string(stdout), "/")
			numerator, err := strconv.Atoi(strings.TrimSpace(parts[0]))
			if err != nil {
				log.Printf("Error: %v", err)
				return nil
			}
			denominator, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil {
				log.Printf("Error: %v", err)
				return nil
			}
			out := fmt.Sprintf("W: %s (%d%%)", w.SSID, (numerator*100)/denominator)
			return outputs.Text(out).
				Color(colors.Scheme("good")).
				Padding(padding).
				OnClick(
				func(event bar.Event) {
					switch event.Button {
					case bar.ButtonLeft:
						restartWifi(*config, *iface)
					case bar.ButtonRight:
						stopWifi()
					default:
						log.Printf("Event button %v not handled", event.Button)
					}
				})
				// OnClick(click.Left(func() { restartWifi(*config, *iface) })).
				// OnClick(click.Right(func() { stopWifi() }))
		case w.Connecting():
			return outputs.Text("W: connecting...").
				Color(colors.Scheme("degraded")).
				Padding(padding)
		case w.Enabled():
			return outputs.Text("W: down").
				Color(colors.Scheme("bad")).
				Padding(padding).
				OnClick(click.Left(func() { restartWifi(*config, *iface) }))
		default:
			return nil
		}
	}))

	barista.Add(netinfo.Prefix("e").Output(func(s netinfo.State) bar.Output {
		switch {
		case s.Connected():
			ip := "<no ip>"
			if len(s.IPs) > 0 {
				ip = s.IPs[0].String()
			}
			return outputs.Textf("E: %s", ip).Color(colors.Scheme("good")).Padding(padding)
		case s.Connecting():
			return outputs.Text("E: connecting...").Color(colors.Scheme("degraded")).Padding(padding)
		case s.Enabled():
			return outputs.Text("E: down").Color(colors.Scheme("bad")).Padding(padding)
		default:
			return nil
		}
	}))

	statusName := map[battery.Status]string{
		battery.Charging:    "CHR",
		battery.Discharging: "BAT",
		battery.NotCharging: "NOT",
		battery.Unknown:     "UNK",
		battery.Full:        "FULL",
	}
	barista.Add(battery.All().Output(func(b battery.Info) bar.Output {
		if b.Status == battery.Disconnected {
			return nil
		}

		out := outputs.Textf("Bat: %d%% (%s)",
			b.RemainingPct(),
			statusName[b.Status],
		)
		if b.Discharging() {
			if b.RemainingPct() < 20 || b.RemainingTime() < 30*time.Minute {
				out.Color(colors.Scheme("bad"))
			}
		}
		return out.Padding(padding)
	}))

	barista.Add(clock.Local().Output(time.Second, func(t time.Time) bar.Output {
		fmtdTime := t.Format("2006-01-02 15:04:05  ")
		return outputs.Text(fmtdTime)
	}))

	panic(barista.Run())
}
