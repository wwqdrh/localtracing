package timecost

import (
	"bytes"
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

var (
	// 时间消耗分段配置表
	costSplitConf = []costSplit{
		{max: time.Second, step: time.Millisecond * 10},
		{min: time.Second, max: time.Second * 2, step: time.Millisecond * 20},
		{min: time.Second * 2, max: time.Second * 5, step: time.Millisecond * 30},
		{min: time.Second * 5, max: time.Second * 10, step: time.Millisecond * 50},
		{min: time.Second * 10, max: time.Minute, step: time.Second},
		{min: time.Minute},
	}

	costTable []*tree
	lock      = &sync.Mutex{}
)

// 时间消耗分段配置
type costSplit struct {
	min  time.Duration //分段开始值，首段从0开始
	max  time.Duration //分段结束值，末段为0
	step time.Duration //段内切割粒度
}

type tree struct {
	name    string
	root    *node
	costAvg time.Duration //真实的平均耗时
	cost50  time.Duration //分段的50分位
	cost99  time.Duration //分段的99分位
	costMax time.Duration //真实的最大耗时
	count   int64         //计数个数
	sync.Mutex
}

type node struct {
	lchild  *node
	rchild  *node
	height  int //树的深度
	level   time.Duration
	avgCost time.Duration
	count   int64
}

// 聚合统计操作耗时，在重要的操作开始处调用，并在后续通过defer调用闭包匿名方法计算时间消耗
func AggHandleTime(name string) func() {
	start := time.Now()
	return func() {
		AggCost(name, time.Since(start))
	}
}

func AggCost(name string, cost time.Duration) {
	ok, size := tryAgg(name, cost)
	if ok {
		return
	}

	t := insertTree(name, size)
	t.insert(cost)
}

func GetCost(name string) *tree {
	for _, t := range costTable {
		if t.name == name {
			t.loadResult()
			return t
		}
	}

	return nil
}

func GetAllCost() []*tree {
	for _, t := range costTable {
		t.loadResult()
	}
	return costTable
}

func (t *tree) MarshalJSON() ([]byte, error) {
	if t == nil {
		return []byte(`{"costAvg":"0ms", "cost50":"0ms", "cost99":"0ms", "costMax":"0ms", "count":0}`), nil
	}
	buf := bytes.NewBufferString(fmt.Sprintf(`{"route":"%v","costAvg":"%v", "cost50":"%v", "cost99":"%v", "costMax":"%v", "count":%d}`, t.name, t.costAvg, t.cost50, t.cost99, t.costMax, t.count))
	return buf.Bytes(), nil
}

func tryAgg(name string, cost time.Duration) (bool, int) {
	size := len(costTable)
	for _, t := range costTable {
		if t.name == name {
			t.insert(cost)
			return true, 0
		}
	}
	return false, size
}

func insertTree(name string, size int) *tree {
	lock.Lock()
	defer lock.Unlock()

	if len(costTable) > size {
		for _, t := range costTable[size:] {
			if t.name == name {
				return t
			}
		}
	}

	t := &tree{name: name}
	costTable = append(costTable, t)

	return t
}

func (t *tree) loadResult() {
	if t == nil {
		return
	}
	var (
		number50 = t.count - int64(math.Round(float64(t.count)/2))
		number99 = t.count - int64(math.Round(float64(t.count)*99/100))
		c50, c99 int64
	)

	log.Printf("n50: %d, n99: %d\n", number50, number99)

	// 右序找
	t.rplWalk(func(p *node) bool {
		if c99 < number99 {
			c99 += p.count
			if c99 >= number99 {
				t.cost99 = p.avgCost
			}
		}
		if c50 < number50 {
			c50 += p.count
		}
		if c50 >= number50 {
			t.cost50 = p.avgCost
			return false
		}
		return true
	})

	if t.cost50 == 0 {
		t.cost50 = t.costAvg
	}
	if t.cost99 == 0 {
		t.cost99 = t.costMax
	}
}

//插入节点 ---> 依次向上递归，调整树平衡
func (t *tree) insert(cost time.Duration) {
	t.Lock()
	defer t.Unlock()

	if cost > t.costMax {
		t.costMax = cost
	}
	t.count++
	t.costAvg = (t.costAvg*(time.Duration(t.count)-1) + cost) / time.Duration(t.count)
	for _, c := range costSplitConf {
		if c.match(cost) {
			t.root = insert(t.root, c.getLevel(cost), cost)
			return
		}
	}
}

func (t *tree) lprWalk(do func(p *node) bool) {
	if t.root == nil {
		return
	}

	ok := true
	t.root.lprWalk(do, &ok)
}

func (t *tree) rplWalk(do func(p *node) bool) {
	if t.root == nil {
		return
	}

	ok := true
	t.root.rplWalk(do, &ok)
}

func max(data1 int, data2 int) int {
	if data1 > data2 {
		return data1
	}
	return data2
}

func getHeight(n *node) int {
	if n == nil {
		return 0
	}
	return n.height
}

// 左旋转
//
//    n  BF = 2
//       \\
//         prchild     ----->       prchild    BF = 1
//           \\                        /   \\
//           pprchild               n  pprchild
func llRotation(n *node) *node {
	prchild := n.rchild
	n.rchild = prchild.lchild
	prchild.lchild = n
	//更新节点 n 的高度
	n.height = max(getHeight(n.lchild), getHeight(n.rchild)) + 1
	//更新新父节点高度
	prchild.height = max(getHeight(prchild.lchild), getHeight(prchild.rchild)) + 1
	return prchild
}

// 右旋转
//             n  BF = -2
//              /
//         plchild     ----->       plchild    BF = 1
//            /                        /   \\
//        pplchild                lchild   n

func rrRotation(n *node) *node {
	plchild := n.lchild
	n.lchild = plchild.rchild
	plchild.rchild = n
	n.height = max(getHeight(n.lchild), getHeight(n.rchild)) + 1
	plchild.height = max(getHeight(n), getHeight(plchild.lchild)) + 1
	return plchild
}

// 先左转再右转
//          n                  n
//         /            左          /     右
//      node1         ---->    node2     --->         node2
//          \\                   /                     /   \\
//          node2s           node1                 node1  n
func lrRotation(n *node) *node {
	plchild := llRotation(n.lchild) //左旋转
	n.lchild = plchild
	return rrRotation(n)

}

// 先右转再左转
//       n                  n
//          \\          右         \\         左
//          node1    ---->       node2     --->      node2
//          /                       \\                /   \\
//        node2                    node1           n  node1
func rlRotation(n *node) *node {
	prchild := rrRotation(n.rchild)
	n.rchild = prchild
	n.rchild = prchild
	return llRotation(n)
}

//处理节点高度问题
func handleBF(n *node) *node {
	if getHeight(n.lchild)-getHeight(n.rchild) == 2 {
		if getHeight(n.lchild.lchild)-getHeight(n.lchild.rchild) > 0 { //RR
			n = rrRotation(n)
		} else {
			n = lrRotation(n)
		}
	} else if getHeight(n.lchild)-getHeight(n.rchild) == -2 {
		if getHeight(n.rchild.lchild)-getHeight(n.rchild.rchild) < 0 { //LL
			n = llRotation(n)
		} else {
			n = rlRotation(n)
		}
	}
	return n
}

func insert(n *node, level, cost time.Duration) *node {
	if n == nil {
		return &node{lchild: nil, rchild: nil, level: level, avgCost: cost, count: 1, height: 1}
	}
	if n.level > level {
		n.lchild = insert(n.lchild, level, cost)
		n = handleBF(n)
	} else if n.level < level {
		n.rchild = insert(n.rchild, level, cost)
		n = handleBF(n)
	} else {
		n.count++
		n.avgCost = (n.avgCost*time.Duration(n.count-1) + cost) / time.Duration(n.count)
		return n
	}
	n.height = max(getHeight(n.lchild), getHeight(n.rchild)) + 1
	return n
}

// right -> parent -> left
func (n *node) rplWalk(do func(p *node) bool, ok *bool) {
	if n == nil {
		return
	}

	if n.rchild != nil {
		n.rchild.rplWalk(do, ok)
		if !*ok {
			return
		}
	}
	*ok = do(n)
	if !*ok {
		return
	}
	if n.lchild != nil {
		n.lchild.rplWalk(do, ok)
		if !*ok {
			return
		}
	}
}

// left -> parent -> right
func (n *node) lprWalk(do func(p *node) bool, ok *bool) {
	if n == nil {
		return
	}

	if n.lchild != nil {
		n.lchild.lprWalk(do, ok)
		if !*ok {
			return
		}
	}
	*ok = do(n)
	if !*ok {
		return
	}
	if n.rchild != nil {
		n.rchild.lprWalk(do, ok)
		if !*ok {
			return
		}
	}
}

func (c *costSplit) match(d time.Duration) bool {
	if d < c.min {
		return false
	}
	if c.max > 0 && d >= c.max {
		return false
	}

	return true
}

func (c *costSplit) getLevel(d time.Duration) time.Duration {
	if c.step > 0 {
		return c.min + (d-c.min)/c.step*c.step
	}

	return c.min
}
