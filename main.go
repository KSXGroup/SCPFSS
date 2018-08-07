package main

import (
	"SCPFSS"
	"strconv"
	"time"
)

func main() {
	/*t := SCPFSS.NewSCPFSS()
	t.RunConsole()*/
	t := new(SCPFSS.ProgressBar)
	t.Title = "Example"
	t.IfTitle = true
	t.IfRight = true
	t.Show()
	for i := 1; i <= 100; i += 1 {
		time.Sleep(time.Millisecond * 200)
		t.Percent = uint8(i)
		t.Right = strconv.Itoa(i) + "%"
		t.Update()
	}
}
