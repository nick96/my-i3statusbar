package main

import (
	"fmt"
	"time"

	"os/exec"
	"strconv"
	"strings"

	"flag"

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
)

const (
	padding = 25
)

func main() {
	email := flag.String("email", "", "Email for LastPass account")
	flag.Parse()

	colors.LoadFromMap(map[string]string{
		"good":     "#0f0",
		"bad":      "#f00",
		"degraded": "#ff0",
	})

	if *email != "" {
		barista.Add(shell.New("bash", "-c", "lpass status || exit 0").Output(func(output string) bar.Output {
			out := outputs.Text("LastPass").Padding(padding)
			if strings.Contains(output, "Logged in") {
				return out.Color(colors.Scheme("good"))
			}
			return out.Color(colors.Scheme("bad")).
				OnClick(click.RunLeft("lpass", "login", *email))
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
			cmd := exec.Command("bash", "-c", shellCmd)
			stdout, err := cmd.Output()
			if err != nil {
				return nil
			}
			parts := strings.Split(string(stdout), "/")
			numerator, err := strconv.Atoi(strings.TrimSpace(parts[0]))
			if err != nil {
				return nil
			}
			denominator, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil {
				return nil
			}
			out := fmt.Sprintf("W: %s (%d%%)", w.SSID, (numerator*100)/denominator)
			return outputs.Text(out).Color(colors.Scheme("good")).Padding(padding)
		case w.Connecting():
			return outputs.Text("W: connecting...").Color(colors.Scheme("degraded")).Padding(padding)
		case w.Enabled():
			return outputs.Text("W: down").Color(colors.Scheme("bad")).Padding(padding)
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
