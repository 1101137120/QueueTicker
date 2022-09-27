package Order

import "github.com/shopspring/decimal"

type Order struct {
	OrderId    string
	Price      decimal.Decimal
	Quantity   decimal.Decimal
	CreateTime int64
	Index      int
	OrderType  OrderType
	PriceType  PriceType
	Amount     decimal.Decimal
}

func NewOrderItem(pt PriceType, ot OrderType, uniqId string, price, quantity, amount decimal.Decimal, createTime int64) *Order {
	return &Order{
		OrderId:    uniqId,
		Price:      price,
		Quantity:   quantity,
		CreateTime: createTime,
		PriceType:  pt,
		Amount:     amount,
		OrderType:  ot,
	}
}

func (o *Order) SetQuantity(qnt decimal.Decimal) {
	o.Quantity = qnt
}

type OrderType int
type PriceType int

const (
	PriceLimit  PriceType = 0
	PriceMarket PriceType = 1
)

const (
	OrderBuy  OrderType = 0
	OrderSell OrderType = 1
)
