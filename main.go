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
	id         string
	parentID   string
	tags       string
	size       string
	created    string
	children   []*node
	containers []string
	depth      int
	isRoot     bool
	isEnd      bool // 排在兄弟节点的最后一个
}

type printContext struct {
	maxDepth      int
	maxTagsLen    int
	maxSizeLen    int
	maxCreatedLen int
	printSize     bool
	printCreated  bool
}

// 移除中间节点：
//   - 没有名字和标签
//   - 没有被容器实例直接依赖
//   - 有子节点
func (n *node) removeIntermediates() {
	for i := 0; i < len(n.children); {
		child := n.children[i]
		if child.tags == "<none>:<none>" && len(child.containers) == 0 && len(child.children) > 0 {
			// 把当前的 child 用 child.children 替换
			tmp := n.children
			n.children = append([]*node{}, tmp[:i]...)
			n.children = append(n.children, child.children...)
			n.children = append(n.children, tmp[i+1:]...)
			continue
		}
		i++
	}

	for _, child := range n.children {
		child.removeIntermediates()
	}
}

func (n *node) isLeaf() bool {
	return len(n.children) == 0
}

func (n *node) profileOutline(ctx *printContext) {
	if ctx.maxDepth < n.depth {
		ctx.maxDepth = n.depth
	}
	if ctx.maxTagsLen < len(n.tags) && (len(n.containers) > 0 || ctx.printSize || ctx.printCreated) {
		ctx.maxTagsLen = len(n.tags)
	}
	if ctx.maxSizeLen < len(n.size) {
		ctx.maxSizeLen = len(n.size)
	}
	if ctx.maxCreatedLen < len(n.created) {
		ctx.maxCreatedLen = len(n.created)
	}
	for i, child := range n.children {
		child.depth = n.depth + 1
		child.isEnd = i == len(n.children)-1

		child.profileOutline(ctx)
	}
}

func (n *node) printTree(prefix, branch string, ctx *printContext) {
	title := ""
	padding := ""
	if n.isRoot {
		title = "."
	} else {
		if n.isLeaf() {
			padding = strings.Repeat("──", ctx.maxDepth-n.depth)
		} else {
			padding = "┬─" + strings.Repeat("──", ctx.maxDepth-n.depth-1)
		}

		// image id
		title = " " + n.id[7:19]

		// image tags
		format := " %-" + strconv.Itoa(ctx.maxTagsLen) + "s"
		if n.tags == "<none>:<none>" {
			if n.isLeaf() {
				title += fmt.Sprintf(format, "*")
			} else {
				title += fmt.Sprintf(format, "-")
			}
		} else {
			title += fmt.Sprintf(format, n.tags)
		}

		// image size
		if ctx.printSize {
			title += fmt.Sprintf("  %"+strconv.Itoa(ctx.maxSizeLen)+"s", n.size)
		}

		// image created
		if ctx.printCreated {
			title += fmt.Sprintf("  %"+strconv.Itoa(ctx.maxCreatedLen)+"s", n.created)
		}

		// containers
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
		child.printTree(childPrefix, childBranch, ctx)
	}
}

func convSizeToReadable(size int64) string {
	var v float32
	v = float32(size) / (1000.0 * 1000.0)
	return fmt.Sprintf("%2.f", v)
}

func convCreatedToReadable(created int64) string {
	return fmt.Sprintf("%d", created)
}

func main() {
	skipIntermediate := true
	printSize := false
	printCreated := false
	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "-a" {
			skipIntermediate = false
		} else if os.Args[i] == "-s" {
			printSize = true
		} else if os.Args[i] == "-c" {
			printCreated = true
		} else {
			println("Usage: ditree [-a] [-s] [-c]")
			println("    -a print all images, default skip intermediate images")
			println("    -s print image size")
			println("    -c print image created time")
			return
		}
	}
	if len(os.Args) > 1 && os.Args[1] == "-a" {
		skipIntermediate = false
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}

	// 查询出所有的 image
	images, err := cli.ImageList(context.Background(), types.ImageListOptions{All: true})
	if err != nil {
		panic(err)
	}

	// 构建 image 之间的父子关系
	root := &node{
		id:         "images",
		parentID:   "",
		tags:       "",
		children:   []*node{},
		containers: []string{},
		depth:      1,
		isRoot:     true,
		isEnd:      false,
	}
	m := map[string]*node{}
	m[root.id] = root
	for _, image := range images {
		// 获取“真正的” parent image
		insp, _, err := cli.ImageInspectWithRaw(context.Background(), image.ID)
		if err != nil {
			panic(err)
		}

		n := &node{
			id:       image.ID,
			parentID: insp.Config.Image,
			tags:     strings.Join(image.RepoTags, ", "),
			size:     convSizeToReadable(image.Size),
			created:  convCreatedToReadable(image.Created),
			children: []*node{},
		}
		m[n.id] = n

	}
	for _, image := range images {
		n := m[image.ID]
		parent, ok := m[n.parentID]
		if ok {
			parent.children = append(parent.children, n)
		} else {
			root.children = append(root.children, n)
		}
	}

	// 查询出所有的 container
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
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
	ctx := &printContext{
		printSize:    printSize,
		printCreated: printCreated,
	}
	root.profileOutline(ctx)
	root.printTree("", "", ctx)
}
