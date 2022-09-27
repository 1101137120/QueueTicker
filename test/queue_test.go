package test

import (
	"fmt"
	"testing"

	. "github.com/User/internal/pkg/Order"
	. "github.com/User/internal/pkg/Queue"
	"github.com/shopspring/decimal"
)

var askQueue *OrderQueue
var bidQueue *OrderQueue
var testTicker = NewQueueTicker("Test")

func d(f float64) decimal.Decimal {
	return decimal.NewFromFloat(f)
}

func init() {
	askQueue = NewQueue()
	bidQueue = NewQueue()
}

func TestAskQueue(t *testing.T) {

	askQueue.En(Order{OrderId: "1", Quantity: d(10), Price: d(10), CreateTime: 1111111, OrderType: OrderSell, PriceType: PriceLimit})
	askQueue.En(Order{OrderId: "2", Quantity: d(20), Price: d(20), CreateTime: 1111111, OrderType: OrderSell, PriceType: PriceLimit})
	askQueue.En(Order{OrderId: "3", Quantity: d(30), Price: d(30), CreateTime: 1111111, OrderType: OrderSell, PriceType: PriceLimit})
	askQueue.En(Order{OrderId: "4", Quantity: d(40), Price: d(40), CreateTime: 1111111, OrderType: OrderSell, PriceType: PriceLimit})
	fmt.Printf("%+v\n", askQueue.Pq)
	top := askQueue.Top()
	fmt.Printf("%+v\n", top)

	var topIndex = askQueue.GetIndexById(top.OrderId)
	fmt.Printf("%+v\n", topIndex)
	top.SetQuantity(d(80))
	fmt.Printf("%+v\n", top)

	askQueue.Remove(1)
	fmt.Printf("%+v\n", askQueue.Pq)
}

func TestTicker(t *testing.T) {
	testTicker.PushNewOrder(Order{OrderId: "1", Quantity: d(10), Price: d(10), CreateTime: 1111111, OrderType: OrderSell, PriceType: PriceLimit})
	fmt.Printf("%+v\n", testTicker.AskLen())
	testTicker.PushNewOrder(Order{OrderId: "1", Quantity: d(10), Price: d(20), CreateTime: 1111111, OrderType: OrderSell, PriceType: PriceLimit})
	testTicker.PushNewOrder(Order{OrderId: "1", Quantity: d(10), Price: d(40), CreateTime: 1111111, OrderType: OrderSell, PriceType: PriceLimit})
	testTicker.PushNewOrder(Order{OrderId: "1", Quantity: d(10), Price: d(30), CreateTime: 1111111, OrderType: OrderSell, PriceType: PriceLimit})
	fmt.Printf("%+v\n", testTicker.AskLen())
	testTicker.PushNewOrder(Order{OrderId: "1", Quantity: d(10), Price: d(10), CreateTime: 1111111, OrderType: OrderBuy, PriceType: PriceLimit})
	fmt.Printf("%+v\n", testTicker.BidLen())
}
