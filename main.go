package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

type node struct {
	ID         string
	ParentID   string
	RepoTags   string
	children   []*node
	containers []string
	depth      int
	isRoot     bool
	isLeaf     bool
	isEnd      bool // 排在兄弟节点的最后一个
}

type outline struct {
	maxDepth        int
	maxImageNameLen int
}

// 移除中间节点：
//   - 没有名字和标签
//   - 有子节点
//   - 没有被容器实例直接依赖
func (n *node) removeIntermediates() {
	for i := 0; i < len(n.children); {
		child := n.children[i]
		if child.RepoTags == "<none>:<none>" && len(child.children) > 0 && len(child.containers) == 0 {
			// 把当前的 child 用 child.children 替换
			children := append(n.children[:i], child.children...)
			n.children = append(children, n.children[i+1:]...)
			continue
		}
		i++
	}

	for _, child := range n.children {
		child.removeIntermediates()
	}
}

func (n *node) profileOutline(o *outline) {
	if o.maxDepth < n.depth {
		o.maxDepth = n.depth
	}
	if o.maxImageNameLen < len(n.RepoTags) && len(n.containers) > 0 {
		o.maxImageNameLen = len(n.RepoTags)
	}
	for i, child := range n.children {
		child.depth = n.depth + 1
		child.isLeaf = len(child.children) == 0
		child.isEnd = i == len(n.children)-1

		child.profileOutline(o)
	}
}

func (n *node) printTree(prefix, branch string, o *outline) {
	title := ""
	padding := ""
	if n.isRoot {
		title = "."
	} else {
		if n.isLeaf {
			padding = strings.Repeat("──", o.maxDepth-n.depth)
		} else {
			padding = "┬─" + strings.Repeat("──", o.maxDepth-n.depth-1)
		}
		title = fmt.Sprintf(" %s %-"+strconv.Itoa(o.maxImageNameLen)+"s", n.ID[7:19], n.RepoTags)
		if len(n.containers) > 0 {
			title += "  => " + strings.Join(n.containers, ", ")
		}
	}
	fmt.Printf("%s%s%s%s\n", prefix, branch, padding, title)

	childPrefix := prefix
	if !n.isRoot {
		childPrefix = prefix + "  "
		if !n.isEnd {
			childPrefix = prefix + "│ "
		}
	}
	for _, child := range n.children {
		childBranch := "├─"
		if child.isEnd {
			childBranch = "└─"
		}
		child.printTree(childPrefix, childBranch, o)
	}
}

func main() {
	skipIntermediate := true
	if len(os.Args) > 1 && os.Args[1] == "-a" {
		skipIntermediate = false
	}

	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}

	// 查询出所有的 image
	images, err := cli.ImageList(ctx, types.ImageListOptions{All: true})
	if err != nil {
		panic(err)
	}

	// 构建 image 之间的父子关系
	root := &node{
		ID:         "images",
		ParentID:   "",
		RepoTags:   "",
		children:   []*node{},
		containers: []string{},
		depth:      1,
		isRoot:     true,
		isLeaf:     false,
		isEnd:      false,
	}
	m := map[string]*node{}
	m[root.ID] = root
	for _, image := range images {
		n := &node{
			ID:       image.ID,
			ParentID: image.ParentID,
			RepoTags: strings.Join(image.RepoTags, ", "),
			children: []*node{},
		}
		m[n.ID] = n

	}
	for _, image := range images {
		n := m[image.ID]
		parent, ok := m[n.ParentID]
		if ok {
			parent.children = append(parent.children, n)
		} else {
			root.children = append(root.children, n)
		}
	}

	// 查询出所有的 container
	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		panic(err)
	}

	// 把 container 与对应的 image 关联起来
	for _, container := range containers {
		n := m[container.ImageID]
		for _, name := range container.Names {
			n.containers = append(n.containers, strings.TrimPrefix(name, "/"))
		}
	}

	// 移除中间节点
	if skipIntermediate {
		root.removeIntermediates()
	}

	// 输出树形图
	o := &outline{}
	root.profileOutline(o)
	root.printTree("", "", o)
}
