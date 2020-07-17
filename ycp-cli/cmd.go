package main

import (
	"bufio"
	"code.int-2.me/yuyyi51/YCP"
	"code.int-2.me/yuyyi51/YCP/utils"
	"fmt"
	"github.com/urfave/cli/v2"
	"os"
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
	s := bufio.NewScanner(os.Stdin)
	if client {
		session, err := YCP.Dial(address, port, logger)
		if err != nil {
			fmt.Printf("client dial error: %v", err)
			os.Exit(1)
		}
		total := 0
		go func() {
			for {
				buffer := make([]byte, 1000)
				//fmt.Printf("reading data\n")
				n, _ := session.Read(buffer)
				total += n
				if n != 0 {
					logger.Info("%s", buffer)
				}
			}
		}()
		mockMessage := "This document is subject to BCP 78 and the IETF Trust's Legal\n   Provisions Relating to IETF Documents\n   (http://trustee.ietf.org/license-info) in effect on the date of\n   publication of this document.  Please review these documents\n   carefully, as they describe your rights and restrictions with respect\n   to this document.  Code Components extracted from this document must\n   include Simplified BSD License text as described in Section 4.e of\n   the Trust Legal Provisions and are provided without warranty as\n   described in the Simplified BSD License.\n\n1.  Introduction\n\n   The Transmission Control Protocol (TCP) [Pos81] uses a retransmission\n   timer to ensure data delivery in the absence of any feedback from the\n   remote data receiver.  The duration of this timer is referred to as\n   RTO (retransmission timeout).  RFC 1122 [Bra89] specifies that the\n   RTO should be calculated as outlined in [Jac88].\n\n   This document codifies the algorithm for setting the RTO.  In\n   addition, this document expands on the discussion in Section 4.2.3.1\n   of RFC 1122 and upgrades the requirement of supporting the algorithm\n   from a SHOULD to a MUST.  RFC 5681 [APB09] outlines the algorithm TCP\n   uses to begin sending after the RTO expires and a retransmission is\n   sent.  This document does not alter the behavior outlined in RFC 5681\n   [APB09].\n\n   In some situations, it may be beneficial for a TCP sender to be more\n   conservative than the algorithms detailed in this document allow.\n   However, a TCP MUST NOT be more aggressive than the following\n   algorithms allow.  This document obsoletes RFC 2988 [PA00].\n\n   The key words \"MUST\", \"MUST NOT\", \"REQUIRED\", \"SHALL\", \"SHALL NOT\",\n   \"SHOULD\", \"SHOULD NOT\", \"RECOMMENDED\", \"MAY\", and \"OPTIONAL\" in this\n   document are to be interpreted as described in [Bra97].\n\n2.  The Basic Algorithm\n\n   To compute the current RTO, a TCP sender maintains two state\n   variables, SRTT (smoothed round-trip time) and RTTVAR (round-trip\n   time variation).  In addition, we assume a clock granularity of G\n   seconds.\n\n\n\n\n\nPaxson, et al.               Standards Track                    [Page 2]\n \nRFC 6298          Computing TCP's Retransmission Timer         June 2011\n\n\n   The rules governing the computation of SRTT, RTTVAR, and RTO are as\n   follows:\n\n   (2.1) Until a round-trip time (RTT) measurement has been made for a\n         segment sent between the sender and receiver, the sender SHOULD\n         set RTO <- 1 second, though the \"backing off\" on repeated\n         retransmission discussed in (5.5) still applies.\n\n         Note that the previous version of this document used an initial\n         RTO of 3 seconds [PA00].  A TCP implementation MAY still use\n         this value (or any other value > 1 second).  This change in the\n         lower bound on the initial RTO is discussed in further detail\n         in Appendix A.\n\n   (2.2) When the first RTT measurement R is made, the host MUST set\n\n            SRTT <- R\n            RTTVAR <- R/2\n            RTO <- SRTT + max (G, K*RTTVAR)\n\n         where K = 4.\n\n   (2.3) When a subsequent RTT measurement R' is made, a host MUST set\n\n            RTTVAR <- (1 - beta) * RTTVAR + beta * |SRTT - R'|\n            SRTT <- (1 - alpha) * SRTT + alpha * R'\n\n         The value of SRTT used in the update to RTTVAR is its value\n         before updating SRTT itself using the second assignment.  That\n         is, updating RTTVAR and SRTT MUST be computed in the above\n         order.\n\n         The above SHOULD be computed using alpha=1/8 and beta=1/4 (as\n         suggested in [JK88]).\n\n         After the computation, a host MUST update\n         RTO <- SRTT + max (G, K*RTTVAR)\n\n   (2.4) Whenever RTO is computed, if it is less than 1 second, then the\n         RTO SHOULD be rounded up to 1 second.\n\n         Traditionally, TCP implementations use coarse grain clocks to\n         measure the RTT and trigger the RTO, which imposes a large\n         minimum value on the RTO.  Research suggests that a large\n         minimum RTO is needed to keep TCP conservative and avoid\n         spurious retransmissions [AP99].  Therefore, this specification\n         requires a large minimum RTO as a conservative approach, while\n\n\n\n\nPaxson, et al.               Standards Track                    [Page 3]\n \nRFC 6298          Computing TCP's Retransmission Timer         June 2011\n\n\n         at the same time acknowledging that at some future point,\n         research may show that a smaller minimum RTO is acceptable or\n         superior.\n\n   (2.5) A maximum value MAY be placed on RTO provided it is at least 60\n         seconds.\n\n3.  Taking RTT Samples\n\n   TCP MUST use Karn's algorithm [KP87] for taking RTT samples.  That\n   is, RTT samples MUST NOT be made using segments that were\n   retransmitted (and thus for which it is ambiguous whether the reply\n   was for the first instance of the packet or a later instance).  The\n   only case when TCP can safely take RTT samples from retransmitted\n   segments is when the TCP timestamp option [JBB92] is employed, since\n   the timestamp option removes the ambiguity regarding which instance\n   of the data segment triggered the acknowledgment.\n\n   Traditionally, TCP implementations have taken one RTT measurement at\n   a time (typically, once per RTT).  However, when using the timestamp\n   option, each ACK can be used as an RTT sample.  RFC 1323 [JBB92]\n   suggests that TCP connections utilizing large congestion windows\n   should take many RTT samples per window of data to avoid aliasing\n   effects in the estimated RTT.  A TCP implementation MUST take at\n   least one RTT measurement per RTT (unless that is not possible per\n   Karn's algorithm).\n\n   For fairly modest congestion window sizes, research suggests that\n   timing each segment does not lead to a better RTT estimator [AP99].\n   Additionally, when multiple samples are taken per RTT, the alpha and\n   beta defined in Section 2 may keep an inadequate RTT history.  A\n   method for changing these constants is currently an open research\n   question.\n\n4.  Clock Granularity\n\n   There is no requirement for the clock granularity G used for\n   computing RTT measurements and the different state variables.\n   However, if the K*RTTVAR term in the RTO calculation equals zero, the\n   variance term MUST be rounded to G seconds (i.e., use the equation\n   given in step 2.3).\n\n       RTO <- SRTT + max (G, K*RTTVAR)\n\n   Experience has shown that finer clock granularities (<= 100 msec)\n   perform somewhat better than coarser granularities."
		_, _ = session.Write([]byte(mockMessage))
		for {
			s.Scan()
			message := s.Text()
			fmt.Printf("get input\n")
			_, _ = session.Write([]byte(message))
		}
	} else {
		address := fmt.Sprintf("%s:%d", address, port)
		server := YCP.NewServer(address, logger)
		err := server.Listen()
		if err != nil {
			fmt.Printf("%v\n", err)
		}
		session := server.Accept()
		total := 0
		go func() {
			for {
				buffer := make([]byte, 1000)
				//fmt.Printf("reading data\n")
				n, _ := session.Read(buffer)
				total += n
				if n != 0 {
					logger.Info("%s", buffer)
				}
			}
		}()
		for {
			s.Scan()
			message := s.Text()
			//fmt.Printf("get input\n")
			_, _ = session.Write([]byte(message))
		}
	}
}
