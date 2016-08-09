package main

import (
	"fmt"
	"math/rand"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/uber-go/zap"
	"github.com/yulrizka/bot"
	"github.com/yulrizka/fam100"
	"github.com/yulrizka/fam100/telegram/cmd"
	"golang.org/x/net/context"
)

var (
	log                  zap.Logger
	logLevel             *zap.Level
	minQuorum            = 3 // minimum players to start game
	graphiteURL          = ""
	quorumWait           = 120 * time.Second
	telegramInBufferSize = 10000
	gameInBufferSize     = 10000
	gameOutBufferSize    = 10000
	defaultQuestionLimit = 0
	botName              = "fam100bot"
	startedAt            time.Time
	timeoutChan          = make(chan string, 10000)
	finishedChan         = make(chan string, 10000)
	adminID              = ""
	httpTimeout          = 10 * time.Second
	roundDuration        = 90
	blockProfileRate     = 0
	plugin               = fam100Bot{}

	// compiled time information
	VERSION   = ""
	BUILDTIME = ""
)

type logger struct {
	zap.Logger
}

func (l logger) Error(msg string, fields ...zap.Field) {
	l.Logger.Error(msg, fields...)
	errorCount.Inc(1)
}

func init() {
	log = logger{zap.NewJSON(zap.AddCaller(), zap.AddStacks(zap.FatalLevel))}
	ExtraQuestionSeed = 1
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "fam100",
		Short: "Run fam100 telegram game",
		Run: func(cmd *cobra.Command, args []string) {
			mainFn()
		},
	}
	rootCmd.Flags().StringVar(&botName, "botname", "fam100bot", "bot name")
	rootCmd.Flags().StringVar(&adminID, "admin", "", "admin id")
	rootCmd.Flags().IntVar(&minQuorum, "quorum", 3, "minimal channel quorum")
	rootCmd.Flags().StringVar(&graphiteURL, "graphite", "", "graphite url, empty to disable")
	rootCmd.Flags().IntVar(&roundDuration, "roundDuration", 90, "round duration in second")
	rootCmd.Flags().IntVar(&defaultQuestionLimit, "questionLimit", -1, "set default question limit")
	rootCmd.Flags().IntVar(&blockProfileRate, "blockProfile", 0, "enable go routine blockProfile for profiling rate set to 1000000000 for sampling every sec")

	// todo fix with cobra
	logLevel = zap.LevelFlag("v", zap.InfoLevel, "log level: all, debug, info, warn, error, panic, fatal, none")

	rootCmd.AddCommand(cmd.Leave)
	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}

func mainFn() {
	// setup logger
	log.SetLevel(*logLevel)
	bot.SetLogger(log)
	fam100.SetLogger(log)

	// enable profiling
	go func() {
		if blockProfileRate > 0 {
			runtime.SetBlockProfileRate(blockProfileRate)
			log.Info("runtime.BlockProfile is enabled", zap.Int("rate", blockProfileRate))
		}
		log.Info("http listener", zap.Error(http.ListenAndServe("localhost:5050", nil)))
	}()

	go func() {
		sigchan := make(chan os.Signal, 1)
		signal.Notify(sigchan, os.Interrupt, syscall.SIGTERM)

		<-sigchan
		postEvent("fam100 shutdown", "shutdown", fmt.Sprintf("shutdown version:%s buildtime:%s", VERSION, BUILDTIME))
		log.Info("STOPED", zap.String("version", VERSION), zap.String("buildtime", BUILDTIME))
		os.Exit(0)
	}()

	log.Info("Fam100 STARTED", zap.String("version", VERSION), zap.String("buildtime", BUILDTIME))
	postEvent("startup", "startup", fmt.Sprintf("startup version:%s buildtime:%s", VERSION, BUILDTIME))

	key := os.Getenv("TELEGRAM_KEY")
	if key == "" {
		log.Fatal("TELEGRAM_KEY can not be empty")
	}
	http.DefaultClient.Timeout = httpTimeout
	fam100.RoundDuration = time.Duration(roundDuration) * time.Second

	dbPath := "fam100.db"
	if path := os.Getenv("QUESTION_DB_PATH"); path != "" {
		dbPath = path
	}
	if n, err := InitQuestion(dbPath); err != nil {
		log.Fatal("Failed loading question DB", zap.Error(err))
	} else {
		log.Info("Question loaded", zap.Int("nQuestion", n))
	}
	if defaultQuestionLimit >= 0 {
		DefaultQuestionLimit = defaultQuestionLimit
	}
	log.Info("Question limit ", zap.Int("DefaultQuestionLimit", DefaultQuestionLimit))

	defer func() {
		if r := recover(); r != nil {
			DefaultQuestionDB.Close()
			panic(r)
		}
		DefaultQuestionDB.Close()
	}()

	if err := fam100.DefaultDB.Init(); err != nil {
		log.Fatal("Failed loading DB", zap.Error(err))
	}
	startedAt = time.Now()
	telegram := bot.NewTelegram(key)
	if err := telegram.AddPlugin(&plugin); err != nil {
		log.Fatal("Failed AddPlugin", zap.Error(err))
	}
	initMetrics(plugin)
	plugin.start()

	telegram.Start()
}

type fam100Bot struct {
	// channel to communicate with telegram
	in       chan interface{}
	out      chan bot.Message
	channels map[string]*channel

	// channel to communicate with game
	quit chan struct{}
}

func (*fam100Bot) Name() string {
	return "Fam100Bot"
}

func (b *fam100Bot) Init(out chan bot.Message) (in chan interface{}, err error) {
	b.in = make(chan interface{}, telegramInBufferSize)
	b.out = out
	b.channels = make(map[string]*channel)
	b.quit = make(chan struct{})

	return b.in, nil
}

func (b *fam100Bot) start() {
	go b.handleInbox()
}

func (b *fam100Bot) stop() {
	close(b.quit)
}

// handleInbox handles incomming chat message
func (b *fam100Bot) handleInbox() {
	for {
		select {
		case <-b.quit:
			return
		case rawMsg := <-b.in:
			start := time.Now()
			if rawMsg == nil {
				log.Fatal("handleInbox input channel is closed")
			}
			messageIncomingCount.Inc(1)

			switch msg := rawMsg.(type) {
			case *bot.Message:
				if msg.Date.Before(startedAt) {
					// ignore message that is received before the process started
					log.Debug("message before started at", zap.Object("msg", msg), zap.String("startedAt", startedAt.String()), zap.String("date", msg.Date.String()))
					continue
				}
				log.Debug("handleInbox got message", zap.Object("msg", msg))

				msgType := msg.Chat.Type
				if msgType == bot.Private {
					messagePrivateCount.Inc(1)
					log.Debug("Got private message", zap.Object("msg", msg))
					if msg.From.ID == adminID {
						switch {
						case strings.HasPrefix(msg.Text, "/say"):
							if b.cmdSay(msg) {
								continue
							}
						case strings.HasPrefix(msg.Text, "/channels"):
							if b.cmdChannels(msg) {
								continue
							}
						case strings.HasPrefix(msg.Text, "/broadcast"):
							if b.cmdBroadcast(msg) {
								continue
							}
						}
					}
					continue
				}

				// ## Handle Commands ##
				switch msg.Text {
				case "/join", "/join@" + botName:
					if b.cmdJoin(msg) {
						continue
					}
				case "/score", "/score@" + botName:
					if b.cmdScore(msg) {
						continue
					}
				}

				chanID := msg.Chat.ID
				ch, ok := b.channels[chanID]
				if chanID == "" || !ok {
					log.Debug("channels not found", zap.String("chanID", chanID), zap.Object("msg", msg))
					mainHandleNotFoundTimer.UpdateSince(start)
					continue
				}
				if len(ch.quorumPlayer) < minQuorum {
					// ignore message if game is not started
					mainHandleMinQuorumTimer.UpdateSince(start)
					continue
				}

				// pass message to the fam100 game package
				gameMsg := fam100.TextMessage{
					Player:     fam100.Player{ID: msg.From.ID, Name: msg.From.FullName()},
					Text:       msg.Text,
					ReceivedAt: msg.ReceivedAt,
				}
				ch.game.In <- gameMsg

				log.Debug("sent to game", zap.String("chanID", chanID), zap.Object("msg", msg))
			}

		case chanID := <-timeoutChan:
			// chan failed to get quorum
			delete(b.channels, chanID)
			text := fmt.Sprintf("Permainan dibatalkan, jumlah pemain tidak cukup  😞")
			b.out <- bot.Message{Chat: bot.Chat{ID: chanID}, Text: text, Format: bot.Markdown}
			log.Info("Quorum timeout", zap.String("chanID", chanID))

		case chanID := <-finishedChan:
			delete(b.channels, chanID)
		}
	}
}

// channel represents channels chat rooms
type channel struct {
	ID                string
	game              *fam100.Game
	quorumPlayer      map[string]bool
	players           map[string]string
	startedAt         time.Time
	cancelTimer       context.CancelFunc
	cancelNotifyTimer context.CancelFunc
}

func (c *channel) startQuorumTimer(wait time.Duration, out chan bot.Message) {
	var ctx context.Context
	ctx, c.cancelTimer = context.WithCancel(context.Background())
	go func() {
		endAt := time.Now().Add(quorumWait)
		notify := []int64{30}

		for {
			if len(notify) == 0 {
				timeoutChan <- c.ID
				return
			}
			timeLeft := time.Duration(notify[0]) * time.Second
			tickAt := endAt.Add(-timeLeft)
			notify = notify[1:]

			select {
			case <-ctx.Done(): //canceled
				return
			case <-time.After(tickAt.Sub(time.Now())):
				text := fmt.Sprintf("Waktu sisa %s", timeLeft)
				out <- bot.Message{Chat: bot.Chat{ID: c.ID}, Text: text, Format: bot.Markdown}
			}
		}
	}()
}

//TODO: refactor into simpler function with game context
func (c *channel) startQuorumNotifyTimer(wait time.Duration, out chan bot.Message) {
	var ctx context.Context
	ctx, c.cancelNotifyTimer = context.WithCancel(context.Background())
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
			players := make([]string, 0, len(c.players))
			for _, p := range c.players {
				players = append(players, p)
			}

			text := fmt.Sprintf(
				"<b>%s</b> OK, butuh %d orang lagi, sisa waktu %s",
				escape(strings.Join(players, ", ")),
				minQuorum-len(c.quorumPlayer),
				quorumWait,
			)
			out <- bot.Message{Chat: bot.Chat{ID: c.ID}, Text: text, Format: bot.HTML, DiscardAfter: time.Now().Add(5 * time.Second)}
			c.cancelNotifyTimer = nil
		}
	}()
}
func messageOfTheDay(chanID string) (string, error) {
	msgStr, err := fam100.DefaultDB.ChannelConfig(chanID, "motd", "")
	if err != nil || msgStr == "" {
		msgStr, err = fam100.DefaultDB.GlobalConfig("motd", "")
	}
	if err != nil {
		return "", err
	}
	messages := strings.Split(msgStr, ";")

	return messages[rand.Intn(len(messages))], nil
}
