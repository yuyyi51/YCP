package main

import (
	"bufio"
	"code.int-2.me/yuyyi51/YCP"
	"code.int-2.me/yuyyi51/YCP/utils"
	"code.int-2.me/yuyyi51/ylog"
	"fmt"
	"github.com/urfave/cli/v2"
	"log"
	"net"
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
				Name:  "log_prefix",
				Usage: "log prefix",
				Value: "ycp",
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
			&cli.IntFlag{
				Name:  "loss",
				Usage: "set a loss rate in sending packets(0-100)",
				Value: 0,
			},
			&cli.BoolFlag{
				Name:  "tcp",
				Usage: "use tcp to transfer, used to compare with YCP",
				Value: false,
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
	logPrefix := c.String("log_prefix")
	logger, err := ylog.NewFileLogger("log", logPrefix, ylog.StringToLogLevel(logLevel), 0)
	if err != nil {
		log.Fatalf("%v", err)
	}
	consoleLogger, err := ylog.NewConsoleLogger(ylog.LogLevelInfo, 0)
	if err != nil {
		log.Fatalf("%v", err)
	}
	logger.AddLogChain(consoleLogger)
	file := c.String("file")
	output := c.String("output")
	pprofPort := c.Int("pprof_port")
	loss := c.Int("loss")
	tcp := c.Bool("tcp")
	if pprofPort != 0 {
		go func() {
			logger.Debug("%v", http.ListenAndServe(fmt.Sprintf("localhost:%d", pprofPort), nil))
		}()
	}

	s := bufio.NewScanner(os.Stdin)

	if !tcp {

	}

	var session *YCP.Session
	var conn net.Conn
	if !tcp {
		if client {
			var err error
			session, err = YCP.Dial(address, port, logger)
			if err != nil {
				logger.Fatal("client dial error: %v", err)
			}
			session.SetLossRate(loss)
			conn = session
		} else {
			address := fmt.Sprintf("%s:%d", address, port)
			server := YCP.NewServer(address, logger)
			err := server.Listen()
			if err != nil {
				logger.Fatal("server listen error: %v", err)
			}
			session = server.Accept()
			session.SetLossRate(loss)
			conn = session
		}
	} else {
		if client {
			con, err := net.Dial("tcp", fmt.Sprintf("%s:%d", address, port))
			if err != nil {
				logger.Fatal("client dial error: %v", err)
			}
			conn = con
		} else {
			listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", address, port))
			if err != nil {
				logger.Fatal("server listen error: %v", err)
			}
			con, err := listener.Accept()
			if err != nil {
				if err != nil {
					logger.Fatal("server accept error: %v", err)
				}
			}
			conn = con
		}
	}

	// read routine
	if output == "" {
		go func() {
			for {
				total := 0
				buffer := make([]byte, 1000)
				//fmt.Printf("reading data\n")
				n, _ := conn.Read(buffer)
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
			record := utils.NewCostTimer()
			for {
				buffer := make([]byte, 10240) // 10KB
				n, err := conn.Read(buffer)
				if err != nil {
					logger.Error("read from session error: %v", err)
					_ = conn.Close()
					break
				}
				total += n
				counter += n
				if counter >= 1024*10000 {
					cost := float64(record.Cost().Milliseconds())
					record.Reset()
					dataTrans := float64(1024 * 10000)
					bandwidth := dataTrans / cost * 1000 / 1024 // KB
					if session != nil {
						logger.Info("read %d data now, bandwidth: %.2fKB/s, rtt: %s", total, bandwidth, session.GetRtt())
					} else {
						logger.Info("read %d data now, bandwidth: %.2fKB/s", total, bandwidth)
					}
					counter -= 1024 * 10000
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
			_, _ = conn.Write([]byte(message))
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
		record := utils.NewCostTimer()
		for {
			buffer := make([]byte, 10240) // 10KB
			n, err := reader.Read(buffer)
			if err != nil {
				logger.Error("read from file %s error: %v", file, err)
				_ = conn.Close()
				break
			}
			readTotal += n
			m, err := conn.Write(buffer[:n])
			if err != nil {
				logger.Error("write info session error: %v", err)
				break
			}
			total += m
			counter += m
			if counter >= 1024*10000 {
				cost := float64(record.Cost().Milliseconds())
				record.Reset()
				dataTrans := float64(1024 * 10000)
				bandwidth := dataTrans / cost * 1000 / 1024 // KB
				if session != nil {
					logger.Info("write %d data now, read %d, bandwidth: %.2fKB/s, rtt: %s", total, readTotal, bandwidth, session.GetRtt())
				} else {
					logger.Info("write %d data now, read %d, bandwidth: %.2fKB/s", total, readTotal, bandwidth)
				}
				counter -= 1024 * 10000
			}
		}
		wg := new(sync.WaitGroup)
		wg.Add(1)
		wg.Wait()
		logger.Info("write routine exit")
	}

	return nil
}
