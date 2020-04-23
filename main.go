package main

import (
	//"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"utils/pve-monitoring/funclib"
)

var mw io.Writer
var logger *log.Logger
var appState int
var config *funclib.Config
var err error

//////////////////////   M A I N    ////////////////////
func main() {

	appfilename := os.Args[0]
	extension := filepath.Ext(appfilename)
	configfilename := appfilename[0:len(appfilename)-len(extension)] + ".conf"

	config, err = funclib.ReadConfig(configfilename)
	check(err)

	//logfile, err := os.OpenFile(config.GLOBAL.FileLogPath, os.O_RDWR|os.O_APPEND, 0660)
	//if err != nil {
	//	logfile, err = os.Create(config.GLOBAL.FileLogPath)
	//	if err != nil {
	//		logfile, err = os.OpenFile("/dev/null", os.O_RDWR|os.O_APPEND, 0660)
	//		if len(os.Args) == 1 {
	//			fmt.Printf("Unable write to log file %v\n\n", config.GLOBAL.FileLogPath)
	//		}
	//	}
	//}
	//defer logfile.Close()

	//mw := io.MultiWriter(os.Stdout, logfile)
	mw := io.MultiWriter(os.Stdout, nil)
	logger = log.New(mw, "", log.LstdFlags)
	funclib.Logger = logger

	clusterShot, err := funclib.GetClusterStatus(&config.PVE)
	check(err)
	//fmt.Println(clusterShot)

	statData, err := funclib.ProcessingData(clusterShot)
	check(err)

	if len(os.Args) < 2 {
		t := strconv.Itoa(config.GLOBAL.CheckingTime)
		err := funclib.PrintStat(statData, t)
		check(err)
		os.Exit(0)
	}

	err = funclib.CliMode(statData, config.GLOBAL.HostName)
	check(err)

}

func check(e error) {
	if e != nil {
		logger.Panic(e)
	}
}
