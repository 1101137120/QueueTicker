package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	_ "net/http/pprof"

	"github.com/User/internal/pkg/Order"
	"github.com/User/internal/pkg/Queue"
	"github.com/User/internal/pkg/wss"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var sendMsg chan []byte
var web *gin.Engine
var queueTicker *Queue.QueueTicker
var recentTrade []interface{}

func main() {

	port := flag.String("port", "8080", "port")
	flag.Parse()
	gin.SetMode(gin.DebugMode)

	//trading_engine.Debug = false
	queueTicker = Queue.NewQueueTicker("AA")

	recentTrade = make([]interface{}, 0)

	go func() {
		log.Println(http.ListenAndServe(":6060", nil))
	}()

	startWeb(*port)
}

func startWeb(port string) {
	web = gin.New()
	web.LoadHTMLGlob("../web/*.html")
	web.StaticFS("/static", http.Dir("../web/static"))

	sendMsg = make(chan []byte, 100)

	go pushDepth()
	go watchTradeLog()

	web.GET("/api/depth", depth)
	web.GET("/api/trade_log", trade_log)
	web.POST("/api/new_order", newOrder)
	web.POST("/api/cancel_order", cancelOrder)
	//web.GET("/api/test_rand", testOrder)

	web.GET("/demo", func(c *gin.Context) {
		c.HTML(200, "demo.html", nil)
	})

	//websocket
	{
		wss.HHub = wss.NewHub()
		go wss.HHub.Run()
		go func() {
			for {
				select {
				case data := <-sendMsg:
					wss.HHub.Send(data)
				default:
					time.Sleep(time.Duration(100) * time.Millisecond)
				}
			}
		}()

		web.GET("/ws", wss.ServeWs)
		web.GET("/pong", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"message": "pong",
			})
		})
	}

	web.Run(":" + port)
}

func depth(c *gin.Context) {
	limit := c.Query("limit")
	limitInt, _ := strconv.Atoi(limit)
	if limitInt <= 0 || limitInt > 100 {
		limitInt = 10
	}
	a := queueTicker.GetAskDepth(limitInt)
	b := queueTicker.GetBidDepth(limitInt)

	c.JSON(200, gin.H{
		"ask": a,
		"bid": b,
	})
}

func trade_log(c *gin.Context) {
	c.JSON(200, gin.H{
		"ok": true,
		"data": gin.H{
			"trade_log": recentTrade,
		},
	})
}

func sendMessage(tag string, data interface{}) {
	msg := gin.H{
		"tag":  tag,
		"data": data,
	}
	msgByte, _ := json.Marshal(msg)
	sendMsg <- []byte(msgByte)
}

func watchTradeLog() {
	for {
		select {
		case log, ok := <-queueTicker.ChTradeResult:
			if ok {
				//

				relog := gin.H{
					"TradePrice":    Queue.FormatDecimal2String(log.TradePrice, 4),
					"TradeAmount":   Queue.FormatDecimal2String(log.TradeAmount, 4),
					"TradeQuantity": Queue.FormatDecimal2String(log.TradeQuantity, 4),
					"TradeTime":     log.TradeTime,
					"AskOrderId":    log.AskOrderId,
					"BidOrderId":    log.BidOrderId,
				}
				sendMessage("trade", relog)

				if len(recentTrade) >= 10 {
					recentTrade = recentTrade[1:]
				}
				recentTrade = append(recentTrade, relog)

				//latest price
				sendMessage("latest_price", gin.H{
					"latest_price": Queue.FormatDecimal2String(log.TradePrice, 4),
				})

			}
		case cancelOrderId := <-queueTicker.ChCancelResult:
			sendMessage("cancel_order", gin.H{
				"OrderId": cancelOrderId,
			})
		default:
			time.Sleep(time.Duration(100) * time.Millisecond)
		}

	}
}

func pushDepth() {
	for {
		ask := queueTicker.GetAskDepth(10)
		bid := queueTicker.GetBidDepth(10)

		sendMessage("depth", gin.H{
			"ask": ask,
			"bid": bid,
		})

		time.Sleep(time.Duration(150) * time.Millisecond)
	}
}

func newOrder(c *gin.Context) {
	type args struct {
		OrderId   string `json:"order_id"`
		OrderType string `json:"order_type"`
		PriceType string `json:"price_type"`
		Price     string `json:"price"`
		Quantity  string `json:"quantity"`
		Amount    string `json:"amount"`
	}

	var param args
	c.BindJSON(&param)

	orderId := uuid.NewString()
	param.OrderId = orderId
	price := string2decimal(param.Price)
	quantity := string2decimal(param.Quantity)

	var pt Order.PriceType
	if param.PriceType == "market" {
		param.Price = "0"
		pt = Order.PriceMarket
		if param.Quantity != "" {
			pt = Order.PriceMarket
			//市价按数量买入资产时，需要用户账户所有可用资产数量，测试默认100块
			param.Amount = "100"
			if quantity.Cmp(decimal.NewFromFloat(100000000)) > 0 || quantity.Cmp(decimal.Zero) <= 0 {
				c.JSON(200, gin.H{
					"ok":    false,
					"error": "數量必須大於0，且不能超過 100000000",
				})
				return
			}
		}

		if queueTicker.AskLen() == 0 {
			c.JSON(200, gin.H{
				"ok":    false,
				"error": "未有人掛賣訂單",
			})
			return
		}
	} else {
		pt = Order.PriceLimit
		param.Amount = "0"
		if price.Cmp(decimal.NewFromFloat(100000000)) > 0 || price.Cmp(decimal.Zero) < 0 {
			c.JSON(200, gin.H{
				"ok":    false,
				"error": "價格必須大於等於0，且不能超過 100000000",
			})
			return
		}
		if quantity.Cmp(decimal.NewFromFloat(100000000)) > 0 || quantity.Cmp(decimal.Zero) <= 0 {
			c.JSON(200, gin.H{
				"ok":    false,
				"error": "數量必須大於0，且不能超過 100000000",
			})
			return
		}
	}

	if strings.ToLower(param.OrderType) == "ask" {
		param.OrderId = fmt.Sprintf("a-%s", orderId)
		item := Order.NewOrderItem(pt, Order.OrderSell, param.OrderId, string2decimal(param.Price), string2decimal(param.Quantity), string2decimal(param.Amount), time.Now().UnixNano())
		queueTicker.ChOrder <- *item

	} else {
		param.OrderId = fmt.Sprintf("b-%s", orderId)
		item := Order.NewOrderItem(pt, Order.OrderBuy, param.OrderId, string2decimal(param.Price), string2decimal(param.Quantity), string2decimal(param.Amount), time.Now().UnixNano())
		queueTicker.ChOrder <- *item
	}

	go sendMessage("new_order", param)

	c.JSON(200, gin.H{
		"ok": true,
		"data": gin.H{
			"ask_len": queueTicker.AskLen(),
			"bid_len": queueTicker.BidLen(),
		},
	})
}

/*func testOrder(c *gin.Context) {
	op := strings.ToLower(c.Query("op_type"))
	if op != "ask" {
		op = "bid"
	}

	func() {
		cnt := 10
		for i := 0; i < cnt; i++ {
			orderId := uuid.NewString()
			if op == "ask" {
				orderId = fmt.Sprintf("a-%s", orderId)
				item := trading_engine.NewAskLimitItem(orderId, randDecimal(20, 50), randDecimal(20, 100), time.Now().UnixNano())
				btcusdt.ChNewOrder <- item
			} else {
				orderId = fmt.Sprintf("b-%s", orderId)
				item := trading_engine.NewBidLimitItem(orderId, randDecimal(1, 20), randDecimal(20, 100), time.Now().UnixNano())
				btcusdt.ChNewOrder <- item
			}

		}
	}()

	c.JSON(200, gin.H{
		"ok": true,
		"data": gin.H{
			"ask_len": btcusdt.AskLen(),
			"bid_len": btcusdt.BidLen(),
		},
	})
}*/

func cancelOrder(c *gin.Context) {
	type args struct {
		OrderId string `json:"order_id"`
	}

	var param args
	c.BindJSON(&param)

	if param.OrderId == "" {
		c.Abort()
		return
	}
	if strings.HasPrefix(param.OrderId, "a-") {
		queueTicker.CancelOrder(Order.OrderSell, param.OrderId)
	} else {
		queueTicker.CancelOrder(Order.OrderBuy, param.OrderId)
	}

	go sendMessage("cancel_order", param)

	c.JSON(200, gin.H{
		"ok": true,
	})
}

func string2decimal(a string) decimal.Decimal {
	d, _ := decimal.NewFromString(a)
	return d
}

func randDecimal(min, max int64) decimal.Decimal {
	rand.Seed(time.Now().UnixNano())

	d := decimal.New(rand.Int63n(max-min)+min, 0)
	return d
}
