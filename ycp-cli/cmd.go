package main

import (
	"bufio"
	"code.int-2.me/yuyyi51/YCP"
	"code.int-2.me/yuyyi51/YCP/utils"
	"fmt"
	"github.com/urfave/cli/v2"
	"net/http"
	_ "net/http/pprof"
	"os"
	"sync"
)

func createApp() *cli.App {
	app := &cli.App{
		Name:  "ycp-cli",
		Usage: "transport data with YCP",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:    "port",
				Aliases: []string{"p"},
				Value:   7654,
				Usage:   "connect or listen the `PORT`",
			},
			&cli.BoolFlag{
				Name:    "client",
				Aliases: []string{"c"},
				Usage:   "start with client mode",
			},
			&cli.StringFlag{
				Name:     "address",
				Aliases:  []string{"a"},
				Usage:    "connect or listen the `ADDRESS`",
				Required: true,
			},
			&cli.StringFlag{
				Name:    "log_level",
				Aliases: []string{"l"},
				Usage:   "log level, could be trace, debug, info, notice, warn, error, fatal, none",
				Value:   "info",
			},
			&cli.StringFlag{
				Name:    "file",
				Aliases: []string{"f"},
				Usage:   "when connection established, start sending `FILE`",
				Value:   "",
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "save data transformed into `FILE`",
				Value:   "",
			},
			&cli.IntFlag{
				Name:    "pprof_port",
				Aliases: []string{"pp"},
				Usage:   "start a pprof server with `PORT`",
				Value:   0,
			},
		},
		Action: appAction,
	}
	return app
}

func appAction(c *cli.Context) error {
	port := c.Int("port")
	address := c.String("address")
	client := c.Bool("client")
	logLevel := c.String("log_level")
	logger := utils.NewLogger(logLevel, 2)
	file := c.String("file")
	output := c.String("output")
	pprofPort := c.Int("pprof_port")
	if pprofPort != 0 {
		go func() {
			logger.Debug("%v", http.ListenAndServe(fmt.Sprintf("localhost:%d", pprofPort), nil))
		}()
	}

	s := bufio.NewScanner(os.Stdin)

	var session *YCP.Session
	if client {
		var err error
		session, err = YCP.Dial(address, port, logger)
		if err != nil {
			logger.Fatal("client dial error: %v", err)
		}
	} else {
		address := fmt.Sprintf("%s:%d", address, port)
		server := YCP.NewServer(address, logger)
		err := server.Listen()
		if err != nil {
			logger.Fatal("server listen error: %v", err)
		}
		session = server.Accept()
	}

	// read routine
	if output == "" {
		go func() {
			for {
				total := 0
				buffer := make([]byte, 1000)
				//fmt.Printf("reading data\n")
				n, _ := session.Read(buffer)
				total += n
				if n != 0 {
					logger.Info("%s", buffer)
				}
			}
		}()
	} else {
		fs, err := os.OpenFile(output, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0755)
		if err != nil {
			logger.Fatal("open output file %s error: %v", output, err)
		}
		writer := bufio.NewWriter(fs)
		go func() {
			total := 0
			counter := 0
			for {
				buffer := make([]byte, 10240) // 10KB
				n, err := session.Read(buffer)
				if err != nil {
					logger.Error("read from session error: %v", err)
					break
				}
				total += n
				counter += n
				if counter >= 1024*1000 {
					logger.Info("read %d data now", total)
					counter -= 1024 * 1000
				}
				_, err = writer.Write(buffer[:n])
				if err != nil {
					logger.Error("write output file %s error: %v", output, err)
					break
				}
				err = writer.Flush()
				if err != nil {
					logger.Error("flush output file %s error: %v", output, err)
					break
				}
			}
			logger.Info("read routine exit")

		}()
	}

	//write routine
	if file == "" {
		// simple chat server
		for {
			s.Scan()
			message := s.Text()
			fmt.Printf("get input\n")
			_, _ = session.Write([]byte(message))
		}
	} else {
		fs, err := os.OpenFile(file, os.O_CREATE|os.O_RDWR, 0755)
		if err != nil {
			logger.Fatal("open output file %s error: %v", file, err)
		}
		reader := bufio.NewReader(fs)
		total := 0
		counter := 0
		readTotal := 0
		for {
			buffer := make([]byte, 10240) // 10KB
			n, err := reader.Read(buffer)
			if err != nil {
				logger.Error("read from file %s error: %v", file, err)
				break
			}
			readTotal += n
			m, err := session.Write(buffer[:n])
			if err != nil {
				logger.Error("write info session error: %v", err)
				break
			}
			total += m
			counter += m
			if counter >= 1024*1000 {
				logger.Info("write %d data now, read %d", total, readTotal)
				counter -= 1024 * 1000
			}
		}
		wg := new(sync.WaitGroup)
		wg.Add(1)
		wg.Wait()
		logger.Info("write routine exit")
	}

	return nil
}
