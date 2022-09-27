package Queue

import (
	"fmt"
	"sync"
	"time"

	. "github.com/User/internal/pkg/Order"
	"github.com/shopspring/decimal"
)

type TradeResult struct {
	Symbol        string          `json:"symbol"`
	AskOrderId    string          `json:"ask_order_id"`
	BidOrderId    string          `json:"bid_order_id"`
	TradeQuantity decimal.Decimal `json:"trade_quantity"`
	TradePrice    decimal.Decimal `json:"trade_price"`
	TradeAmount   decimal.Decimal `json:"trade_amount"`
	TradeTime     int64           `json:"trade_time"`
}

type QueueTicker struct {
	Symbol         string
	ChOrder        chan Order
	ChTradeResult  chan TradeResult
	ChCancelResult chan string
	latestPrice    decimal.Decimal
	askQueue       *OrderQueue
	bidQueue       *OrderQueue

	sync.Mutex
}

func (t *QueueTicker) PushNewOrder(item Order) {
	t.handlerNewOrder(item)
}

func NewQueueTicker(symbol string) *QueueTicker {
	t := &QueueTicker{
		Symbol:         symbol,
		ChTradeResult:  make(chan TradeResult, 10),
		ChOrder:        make(chan Order),
		ChCancelResult: make(chan string, 10),
		askQueue:       NewQueue(),
		bidQueue:       NewQueue(),
	}
	go t.depthTicker(t.askQueue)
	go t.depthTicker(t.bidQueue)
	go t.matching()
	return t
}

func (t *QueueTicker) AskLen() int {

	return t.askQueue.Pq.Len()
}

func (t *QueueTicker) BidLen() int {

	return t.bidQueue.Pq.Len()
}

func (t *QueueTicker) handlerNewOrder(newOrder Order) {
	t.Lock()
	defer t.Unlock()

	if newOrder.OrderType == OrderSell {
		t.askQueue.En(newOrder)
		t.Sell(newOrder)
	} else {
		t.bidQueue.En(newOrder)
		t.Buy(newOrder)
	}
}

func (t *QueueTicker) GetAskDepth(size int) [][2]string {
	return t.depth(t.askQueue, size)
}

func (t *QueueTicker) GetBidDepth(size int) [][2]string {
	return t.depth(t.bidQueue, size)
}

func (t *QueueTicker) depth(queue *OrderQueue, size int) [][2]string {
	queue.Lock()
	defer queue.Unlock()

	max := len(queue.Depth)
	if size <= 0 || size > max {
		size = max
	}

	return queue.Depth[0:size]
}

func (t *QueueTicker) matching() {
	for {
		select {
		case newOrder := <-t.ChOrder:
			go t.handlerNewOrder(newOrder)
		default:
		}
	}
}

func (t *QueueTicker) CancelOrder(orderType OrderType, uniq string) {
	if orderType == OrderSell {
		t.askQueue.Remove(t.askQueue.GetIndexById(uniq))
	} else {
		t.bidQueue.Remove(t.bidQueue.GetIndexById(uniq))
	}
	t.ChCancelResult <- uniq
}

func (t *QueueTicker) Buy(item Order) {
	for {
		ok := func() bool {
			if t.askQueue.Pq.Len() == 0 || item.Quantity.Equal(decimal.Zero) {
				return false
			}

			if item.PriceType == PriceLimit {
				var isExist, index = t.askQueue.GetIndexByPrice(item.Price)
				if !isExist {
					return false
				}
				var order = t.askQueue.Get(index)
				curTradeQty := decimal.Zero
				if order.Quantity.Cmp(item.Quantity) <= 0 {
					curTradeQty = order.Quantity
					var _, index = t.askQueue.GetIndexByUnId(order.OrderId)
					t.askQueue.Remove(index)
				} else {
					curTradeQty = item.Quantity
					t.askQueue.Pq.SetQuantity(index, order.Quantity.Sub(item.Quantity))
				}
				t.sendTradeResultNotify(order, item, order.Price, curTradeQty)
				item.Quantity = item.Quantity.Sub(curTradeQty)

				return true
			} else {

				ask := t.askQueue.Top()

				var isExist, index = t.askQueue.GetIndexByUnId(ask.OrderId)
				if !isExist {
					return false
				}

				curTradeQty := decimal.Zero
				if ask.Quantity.Cmp(item.Quantity) <= 0 {
					curTradeQty = ask.Quantity
					t.askQueue.Remove(t.askQueue.GetIndexById(ask.OrderId))
				} else {
					curTradeQty = item.Quantity
					t.askQueue.Pq.SetQuantity(index, ask.Quantity.Sub(item.Quantity))
				}

				t.sendTradeResultNotify(ask, item, ask.Price, curTradeQty)
				item.Quantity = item.Quantity.Sub(curTradeQty)
				return true
			}

		}()

		if !ok {
			time.Sleep(time.Duration(200) * time.Millisecond)
			break
		}
	}

	if t.askQueue.Pq.Len() != 0 && item.Quantity.Equal(decimal.Zero) {
		defer t.bidQueue.Remove(t.bidQueue.GetIndexById(item.OrderId))
	}
}

func (t *QueueTicker) sendTradeResultNotify(ask, bid Order, price, tradeQty decimal.Decimal) {
	tradelog := TradeResult{}
	tradelog.Symbol = t.Symbol
	tradelog.AskOrderId = ask.OrderId
	tradelog.BidOrderId = bid.OrderId
	tradelog.TradeQuantity = tradeQty
	tradelog.TradePrice = price
	tradelog.TradeTime = time.Now().Unix()
	tradelog.TradeAmount = tradeQty.Mul(price)

	t.latestPrice = price

	/*if Debug {
		logrus.Infof("%s tradelog: %+v", t.Symbol, tradelog)
	}*/

	t.ChTradeResult <- tradelog
}

func (t *QueueTicker) Sell(item Order) {
	for {
		ok := func() bool {
			if t.bidQueue.Pq.Len() == 0 || item.Quantity.Equal(decimal.Zero) {
				return false
			}

			if item.PriceType == PriceLimit {
				var isExist, index = t.bidQueue.GetIndexByPrice(item.Price)
				if !isExist {
					return false
				}
				var order = t.bidQueue.Get(index)
				curTradeQty := decimal.Zero
				if order.Quantity.Cmp(item.Quantity) <= 0 {
					curTradeQty = order.Quantity
					var _, a = t.bidQueue.GetIndexByUnId(order.OrderId)
					t.bidQueue.Remove(a)
				} else {
					curTradeQty = item.Quantity
					t.bidQueue.Pq.SetQuantity(index, order.Quantity.Sub(item.Quantity))
				}
				t.sendTradeResultNotify(order, item, order.Price, curTradeQty)
				item.Quantity = item.Quantity.Sub(curTradeQty)

				return true
			} else {

				bid := t.bidQueue.Top()

				var isExist, index = t.bidQueue.GetIndexByUnId(bid.OrderId)
				if !isExist {
					return false
				}

				curTradeQty := decimal.Zero
				if bid.Quantity.Cmp(item.Quantity) <= 0 {
					curTradeQty = bid.Quantity
					t.bidQueue.Remove(t.bidQueue.GetIndexById(bid.OrderId))
				} else {
					curTradeQty = item.Quantity
					t.bidQueue.Pq.SetQuantity(index, bid.Quantity.Sub(item.Quantity))
				}

				t.sendTradeResultNotify(bid, item, bid.Price, curTradeQty)
				item.Quantity = item.Quantity.Sub(curTradeQty)
				return true
			}

		}()

		if !ok {
			time.Sleep(time.Duration(200) * time.Millisecond)
			break
		}
	}

	if t.bidQueue.Pq.Len() != 0 && item.Quantity.Equal(decimal.Zero) {
		defer t.askQueue.Remove(t.askQueue.GetIndexById(item.OrderId))
	}
}

func (t *QueueTicker) depthTicker(que *OrderQueue) {

	ticker := time.NewTicker(time.Duration(100) * time.Millisecond)

	for {
		<-ticker.C
		func() {
			que.Lock()
			defer que.Unlock()
			que.Depth = [][2]string{}
			depthMap := make(map[string]string)

			if que.Pq.Len() > 0 {

				for i := 0; i < que.Pq.Len(); i++ {
					item := (*que.Pq)[i]

					price := FormatDecimal2String(item.Price, 2)
					if _, ok := depthMap[price]; !ok {
						depthMap[price] = FormatDecimal2String(item.Quantity, 4)
					} else {
						old_qunantity, _ := decimal.NewFromString(depthMap[price])
						depthMap[price] = FormatDecimal2String(old_qunantity.Add(item.Quantity), 4)
					}
				}

				//按价格排序map
				que.Depth = sortMap2Slice(depthMap, que.Top().OrderType)
			}
		}()
	}
}

func FormatDecimal2String(d decimal.Decimal, digit int) string {
	f, _ := d.Float64()
	format := "%." + fmt.Sprintf("%d", digit) + "f"
	return fmt.Sprintf(format, f)
}

func sortMap2Slice(m map[string]string, ask_bid OrderType) [][2]string {
	res := [][2]string{}
	keys := []string{}
	for k, _ := range m {
		keys = append(keys, k)
	}

	if ask_bid == OrderSell {
		keys = quickSort(keys, "asc")
	} else {
		keys = quickSort(keys, "desc")
	}

	for _, k := range keys {
		res = append(res, [2]string{k, m[k]})
	}
	return res
}

func quickSort(nums []string, asc_desc string) []string {
	if len(nums) <= 1 {
		return nums
	}

	spilt := nums[0]
	left := []string{}
	right := []string{}
	mid := []string{}

	for _, v := range nums {
		vv, _ := decimal.NewFromString(v)
		sp, _ := decimal.NewFromString(spilt)
		if vv.Cmp(sp) == -1 {
			left = append(left, v)
		} else if vv.Cmp(sp) == 1 {
			right = append(right, v)
		} else {
			mid = append(mid, v)
		}
	}

	left = quickSort(left, asc_desc)
	right = quickSort(right, asc_desc)

	if asc_desc == "asc" {
		return append(append(left, mid...), right...)
	} else {
		return append(append(right, mid...), left...)
	}
}
