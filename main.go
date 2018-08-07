package main

import (
	"SCPFSS"
	//"fmt"
	//"time"
)

func main() {
	t := SCPFSS.NewSCPFSS()
	t.RunConsole()
	/*t := SCPFSS.NewProgressBar(5, true, true, "Example")
	t.Show()
	for i := 1; i <= 101; i += 1 {
		time.Sleep(100 * time.Millisecond)
		t.Percent = uint8(i)
		//fmt.Print(t.Percent)
	}
	t.Stop()*/
}
