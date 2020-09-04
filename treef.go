package treef

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
)

var Skip = errors.New("skip")

type Schema struct {
	Left, Rigth uint64
	Name        string
}

func (this Schema) String() string {
	return fmt.Sprintf("%d %d %q", this.Left, this.Rigth, this.Name)
}

func (this Schema) Map() map[string]interface{} {
	return map[string]interface{}{
		"l": this.Left,
		"r": this.Rigth,
		"n": this.Name,
	}
}

type Node struct {
	IdFunc func() []byte
	Left,
	Rigth uint64
	Name       string
	Data, Type interface{}
	Attr       map[string]interface{}

	Children []*Node
	Parent   *Node
}

func (this Node) Path() string {
	var p []string
	for t := &this; t != nil; t = t.Parent {
		p = append(p, t.Name)
	}
	for i, j := 0, len(p)-1; i < j; i, j = i+1, j-1 {
		p[i], p[j] = p[j], p[i]
	}
	return path.Join(p...)
}

func (this *Node) ID() []byte {
	if this.IdFunc == nil {
		h := sha1.New()
		for t := this; t != nil && t.Name != ""; t = t.Parent {
			h.Write([]byte(t.Name))
		}
		b := h.Sum(nil)
		this.IdFunc = func() []byte {
			return b
		}
	}
	return this.IdFunc()
}

func (this Node) fPrint(out io.Writer, depth int64) (n int, err error) {
	if n, err = fmt.Fprintf(out, "{ %d %q [%d] ", this.Left, this.Name, depth); err != nil {
		return
	}
	var n2 int
	if q := len(this.Children); q > 0 {
		n2, err = out.Write([]byte("["))
		n += n2
		if err != nil {
			return
		}

		for i, c := range this.Children {
			n2, err = c.fPrint(out, depth+1)
			n += n2
			if err != nil {
				return
			}
			if i < q-1 {
				n2, err = out.Write([]byte(", "))
				n += n2
				if err != nil {
					return
				}
			}
		}
		n2, err = out.Write([]byte("] "))
		n += n2
		if err != nil {
			return
		}
	}

	n2, err = fmt.Fprintf(out, "%d }", this.Rigth)
	n += n2
	return
}

func (this Node) FPrint(out io.Writer) (n int, err error) {
	var n2 int
	if q := len(this.Children); q > 0 {
		for i, c := range this.Children {
			n2, err = c.fPrint(out, 0)
			n += n2
			if err != nil {
				return
			}
			if i < q-1 {
				n2, err = out.Write([]byte(", "))
				n += n2
				if err != nil {
					return
				}
			}
		}
	}
	return
}

func (this Node) Schema() Schema {
	var s = Schema{
		Left:  this.Left,
		Rigth: this.Rigth,
		Name:  this.Name,
	}
	return s
}

func (this *Node) Add(child ...*Node) *Node {
	if this.Rigth == 0 {
		this.Rigth = 1
	}
	for _, c := range child {
		if children := c.Children; len(children) > 0 {
			c.Children = nil
			c.Add(children...)
		}
		c.Parent = this
	}
	this.Children = append(this.Children, child...)
	sort.Slice(this.Children, func(i, j int) bool {
		return this.Children[i].Name < this.Children[j].Name
	})
	oldRight := this.Rigth
	r := this.update()
	this.Rigth = r
	r = this.Rigth - oldRight

	for p := this.Parent; p != nil; p = p.Parent {
		p.Rigth += r
	}
	return this
}

func (this *Node) update() uint64 {
	var v = this.Left
	if q := len(this.Children); q > 0 {
		for _, c := range this.Children {
			c.Left = v + 1
			v = c.update()
		}
		this.Rigth = this.Children[q-1].Rigth + 1
	} else {
		this.Rigth = this.Left + 1
	}
	return this.Rigth
}

func (this *Node) Remove(child ...*Node) *Node {
	for _, c := range child {
		for i, n := range this.Children {
			if n == c {
				this.Children = append(this.Children[0:i], this.Children[i+1:]...)
				break
			}
		}
		c.Left = 0
		c.Rigth = 1
		children := c.Children
		c.Children = nil
		c.Add(children...)
		c.Parent = nil
	}
	oldRight := this.Rigth
	r := this.update()
	this.Rigth = r
	r = this.Rigth - oldRight

	for p := this.Parent; p != nil; p = p.Parent {
		p.Rigth += r
	}
	return this
}

func (this *Node) Walk(cb func(i, depth int, n *Node) error) (err error) {
	return this.walk(0, cb)
}

func (this *Node) walk(depth int, cb func(i, depth int, n *Node) error) (err error) {
	for i, n := range this.Children {
		if err = cb(i, depth, n); err != nil {
			if err == Skip {
				continue
			}
			return
		}
		if err = n.walk(depth+1, cb); err != nil {
			return
		}
	}
	return
}

func (this *Node) GetOrCreatePath(pth ...string) *Node {
	var pthS []string
	switch len(pth) {
	case 0:
		return this
	case 1:
		pthS = strings.Split(pth[0], "/")
	default:
		pthS = strings.Split(path.Join(pth...), "/")
	}

path_loop:
	for _, name := range pthS {
		if name == "." {
			continue
		}
		for _, c := range this.Children {
			if c.Name == name {
				this = c
				continue path_loop
			}
		}
		// not found
		child := &Node{Name: name}
		this.Add(child)
		this = child
	}
	return this
}

func Attr(n *Node, attr map[string]interface{}) *Node {
	n.Attr = attr
	return n
}
