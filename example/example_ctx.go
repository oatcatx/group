package group

import (
	"errors"
	"fmt"
	"time"
)

// A -> B -> D
// \        /
// -\> C -/ -> E
// F -> X (F will fail)
// time spent (without E) = A + max(B, C) + D = 4
// time spent (with E) = A + max(B, C) + D + E = 5
// res(d) = (1 + 1) + (1 + 1) = 4
type exampleCtx struct {
	a, b, c, d, e, x int
}

func (c *exampleCtx) A() error {
	time.Sleep(1 * time.Second)
	fmt.Println("A")
	c.a = 1
	return nil
}

func (c *exampleCtx) B() error {
	time.Sleep(1 * time.Second)
	fmt.Println("B")
	c.b = c.a + 1
	return nil
}

func (c *exampleCtx) C() error {
	time.Sleep(2 * time.Second)
	fmt.Println("C")
	c.c = c.a + 1
	return nil
}

func (c *exampleCtx) D() error {
	time.Sleep(1 * time.Second)
	fmt.Println("D")
	c.d = c.b + c.c
	return nil
}

func (c *exampleCtx) E() error {
	time.Sleep(1 * time.Second)
	fmt.Println("E")
	c.e = c.c + 1
	return nil
}

func (c *exampleCtx) F() error {
	time.Sleep(1 * time.Second)
	fmt.Println("F")
	return errors.New("F_ERR")
}

func (c *exampleCtx) X() error {
	time.Sleep(1 * time.Second)
	c.x = 1
	return nil
}

func (c *exampleCtx) Res() int {
	return c.d
}
