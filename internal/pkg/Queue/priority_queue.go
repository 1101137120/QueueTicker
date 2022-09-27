package Queue

import (
	"container/heap"
	"sync"

	. "github.com/User/internal/pkg/Order"
	"github.com/shopspring/decimal"
)

type PriorityQueue []Order

type OrderQueue struct {
	Pq    *PriorityQueue
	Depth [][2]string
	sync.Mutex
}

func (p PriorityQueue) Len() int      { return len(p) }
func (p PriorityQueue) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

func (p PriorityQueue) Less(i, j int) bool {
	return (p[i].Price.Cmp(p[j].Price) != -1) || (p[i].Price.Cmp(p[j].Price) == 0 && p[i].CreateTime > p[j].CreateTime)
}
func (p *PriorityQueue) Push(x interface{}) {
	*p = append(*p, x.(Order))
}

func (p *PriorityQueue) Pop() interface{} {
	old := *p
	tmp := old[len(*p)-1]
	*p = old[0 : len(*p)-1]
	return tmp
}

func (p PriorityQueue) SetQuantity(index int, Quantity decimal.Decimal) {
	p[index].SetQuantity(Quantity)
}

func NewQueue() *OrderQueue {
	pq := make(PriorityQueue, 0)
	heap.Init(&pq)
	queue := OrderQueue{
		Pq: &pq,
	}
	return &queue
}

func (o *OrderQueue) GetIndexByUnId(OrderId string) (bool, int) {
	for index, element := range *o.Pq {
		if element.OrderId == OrderId {
			return true, index
		}
	}
	return false, 0
}

func (o *OrderQueue) GetIndexByPrice(Price decimal.Decimal) (bool, int) {
	for index, element := range *o.Pq {
		if element.Price.String() == Price.String() {
			return true, index
		}
	}
	return false, 0
}

func (o *OrderQueue) GetIndexById(OrderId string) int {
	var indexs = 0
	for index, element := range *o.Pq {
		if element.OrderId == OrderId {
			indexs = index
		}
	}
	return indexs
}

func (o *OrderQueue) GetOrderByPrice(Price decimal.Decimal) (bool, Order) {
	var order Order
	for index, element := range *o.Pq {
		if element.Price.String() == Price.String() {
			return true, o.Get(index)
		}
	}

	return false, order
}

func (o *OrderQueue) Remove(index int) {
	heap.Remove(o.Pq, index)
}

func (o *OrderQueue) Get(index int) Order {
	return (*o.Pq)[index]
}

func (o *OrderQueue) Top() Order {
	return o.Get(0)
}

func (p *OrderQueue) En(e Order) {
	heap.Push(p.Pq, e)
}

func (p *OrderQueue) De() {
	heap.Pop(p.Pq)
}
