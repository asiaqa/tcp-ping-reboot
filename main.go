package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	logFileFmt = "%s.log"
)

var (
	address  string
	port     int
	interval time.Duration
	duration time.Duration
	timeout  int
	maxCount int
	help     bool
	hostname string
)

func init() {
	flag.StringVar(&address, "address", "", "Address to ping")
	flag.IntVar(&port, "port", 80, "Port number to use when pinging the address")
	flag.DurationVar(&interval, "interval", 1*time.Minute, "Interval between pings")
	flag.DurationVar(&duration, "duration", 10*time.Minute, "Duration to run program")
	flag.IntVar(&timeout, "timeout", 3, "Number of failed attempts before system reboot")
	flag.IntVar(&maxCount, "max-count", 5, "Maximum number of reboots allowed")
	flag.BoolVar(&help, "help", false, "Displays this help message")
	flag.Parse()

	if help {
		flag.PrintDefaults()
		os.Exit(0)
	}

	if address == "" {
		log.Fatal("Address cannot be empty")
	}

	// Set hostname
	host, err := os.Hostname()
	if err != nil {
		log.Fatalf("Error getting hostname: %v", err)
	}
	hostname = strings.Split(host, ".")[0]
}

func main() {
	// Set up logging to a file in the same directory with the executable file name
	execFilePath, err := os.Executable()
	if err != nil {
		log.Fatalf("Error getting executable file path: %v", err)
	}
	execDirPath := filepath.Dir(execFilePath)
	execFileName := strings.TrimSuffix(filepath.Base(execFilePath), filepath.Ext(execFilePath))
	logFileName := fmt.Sprintf(logFileFmt, execFileName)
	logFilePath := filepath.Join(execDirPath, logFileName)
	logFile, err := os.OpenFile(logFilePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Error opening file: %v", err)
	}
	defer logFile.Close()
	log.SetOutput(logFile)

	// Run the program in the background
	if os.Getenv("BACKGROUND") != "1" {
		cmd := exec.Command(os.Args[0], "-background")
		cmd.Env = append(os.Environ(), "BACKGROUND=1")
		err := cmd.Start()
		if err != nil {
			log.Fatalf("Error starting program: %v", err)
		}
		fmt.Println("Program running in the background...")
		return
	}

	// Check reboot count
	count, err := readRebootCount()
	if err != nil {
		log.Print(err)
		count = 0
	}
	if count >= maxCount {
		log.Printf("Maximum reboot count of %d reached. Exiting...", maxCount)
		return
	}

	start := time.Now()
	for time.Since(start) < duration {
		pingTCP(address, port, timeout)
		time.Sleep(interval)
	}
	log.Printf("Duration of %v reached. Exiting...", duration)
}

func pingTCP(address string, port, timeout int) {
	timeoutStr := strconv.Itoa(timeout)
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", address, port), time.Duration(timeout)*time.Second)
	if err != nil {
		log.Printf("%s: Ping failed for %s: %s\n", time.Now().Format(time.RFC3339), address, err)
		reboot()
	} else {
		conn.Close()
		log.Printf("%s: Ping successful for %s\n", time.Now().Format(time.RFC3339), address)
	}
}

func reboot() {
	count, err := readRebootCount()
	if err != nil {
		log.Print(err)
		count = 0
	}
	if count >= maxCount {
		log.Printf("Maximum reboot count of %d reached. Exiting...", maxCount)
		return
	}
	count++
	if err := writeRebootCount(count); err != nil {
		log.Print(err)
	}
	log.Printf("%s: System reboot initiated. Reboot count: %d", time.Now().Format(time.RFC3339), count)
	cmd := exec.Command("shutdown", "/r", "/t", "0")
	err = cmd.Run()
	if err != nil {
		log.Printf("%s: Error rebooting system: %v", time.Now().Format(time.RFC3339), err)
	}
}

func readRebootCount() (int, error) {

	count := 0	countFile := fmt.Sprintf("%s_reboot_count.txt", hostname)

	countFilePath := filepath.Join(filepath.Dir(os.Args[0]), countFile)

	if _, err := os.Stat(countFilePath); err == nil {

		countBytes, err := ioutil.ReadFile(countFilePath)

		if err != nil {

			return count, fmt.Errorf("Error reading file: %v", err)

		}

		countStr := strings.TrimSpace(string(countBytes))

		count, err = strconv.Atoi(countStr)

		if err != nil {

			return count, fmt.Errorf("Error converting string to integer: %v", err)

		}

	}

	return count, nil

}

func writeRebootCount(count int) error {

	countFile := fmt.Sprintf("%s_reboot_count.txt", hostname)

	countFilePath := filepath.Join(filepath.Dir(os.Args[0]), countFile)

	countStr := strconv.Itoa(count)

	err := ioutil.WriteFile(countFilePath, []byte(countStr), 0644)

	if err != nil {

		return fmt.Errorf("Error writing file: %v", err)

	}

	return nil

}
