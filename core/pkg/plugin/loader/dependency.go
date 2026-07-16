package loader

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"centag/core/pkg/pipeline"
	"centag/core/pkg/plugin/registry"
)

// DependencyResolver 依赖解析器
type DependencyResolver struct {
	registry *registry.Client
	loader   Loader
}

// NewDependencyResolver 创建依赖解析器
func NewDependencyResolver(registry *registry.Client, loader Loader) *DependencyResolver {
	return &DependencyResolver{
		registry: registry,
		loader:   loader,
	}
}

// Resolve 解析依赖
func (r *DependencyResolver) Resolve(ctx context.Context, dependencies []Dependency) (*DependencyGraph, error) {
	graph := NewDependencyGraph()
	
	for _, dep := range dependencies {
		if err := r.resolveDependency(ctx, graph, dep, make(map[string]bool)); err != nil {
			return nil, err
		}
	}
	
	// 检查循环依赖
	if err := graph.DetectCycles(); err != nil {
		return nil, err
	}
	
	// 拓扑排序
	if err := graph.TopologicalSort(); err != nil {
		return nil, err
	}
	
	return graph, nil
}

// resolveDependency 递归解析依赖
func (r *DependencyResolver) resolveDependency(ctx context.Context, graph *DependencyGraph, dep Dependency, visited map[string]bool) error {
	// 检查循环依赖
	if visited[dep.ID] {
		return fmt.Errorf("circular dependency detected: %s", dep.ID)
	}
	visited[dep.ID] = true
	
	// 检查是否已解析
	if graph.Has(dep.ID) {
		return nil
	}
	
	// 从注册中心获取插件
	metadata, err := r.registry.GetLatestVersion(ctx, dep.ID)
	if err != nil {
		if dep.Optional {
			return nil // 可选依赖可以不存在
		}
		return fmt.Errorf("failed to resolve dependency %s@%s: %w", dep.ID, dep.Version, err)
	}
	
	// 添加到图
	node := &DependencyNode{
		ID:       metadata.ID,
		Name:     metadata.Name,
		Version:  metadata.Version,
		Resolved: true,
	}
	graph.Add(node)
	
	// 递归解析依赖的依赖
	for _, subDep := range metadata.Dependencies {
		sub := Dependency{
			ID:       subDep.ID,
			Version:  subDep.Version,
			Optional: subDep.Optional,
		}
		if err := r.resolveDependency(ctx, graph, sub, visited); err != nil {
			return err
		}
		// 添加边
		graph.AddEdge(dep.ID, subDep.ID)
	}
	
	return nil
}

// Install 安装依赖
func (r *DependencyResolver) Install(ctx context.Context, graph *DependencyGraph) error {
	// 按拓扑顺序安装
	for _, node := range graph.SortedNodes {
		// 检查是否已加载
		if _, err := r.loader.Get(node.ID); err == nil {
			continue // 已加载
		}
		
		// 加载依赖
		req := &LoadRequest{
			Source:     "registry",
			PluginID:   node.ID,
			Version:    node.Version,
			AutoStart:  true,
		}
		
		if _, err := r.loader.Load(ctx, req); err != nil {
			return fmt.Errorf("failed to load dependency %s@%s: %w", node.ID, node.Version, err)
		}
	}
	
	return nil
}

// DependencyGraph 依赖图
type DependencyGraph struct {
	Nodes       map[string]*DependencyNode
	Edges       map[string][]string // from -> to
	SortedNodes []*DependencyNode
}

// NewDependencyGraph 创建依赖图
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		Nodes: make(map[string]*DependencyNode),
		Edges: make(map[string][]string),
	}
}

// DependencyNode 依赖节点
type DependencyNode struct {
	ID       string
	Name     string
	Version  string
	Resolved bool
	Depth    int
}

// Add 添加节点
func (g *DependencyGraph) Add(node *DependencyNode) {
	g.Nodes[node.ID] = node
}

// Has 检查节点是否存在
func (g *DependencyGraph) Has(id string) bool {
	return g.Nodes[id] != nil
}

// AddEdge 添加边
func (g *DependencyGraph) AddEdge(from, to string) {
	if g.Edges[from] == nil {
		g.Edges[from] = make([]string, 0)
	}
	// 检查是否已存在
	for _, e := range g.Edges[from] {
		if e == to {
			return
		}
	}
	g.Edges[from] = append(g.Edges[from], to)
}

// DetectCycles 检测循环依赖
func (g *DependencyGraph) DetectCycles() error {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	
	for id := range g.Nodes {
		if !visited[id] {
			if g.detectCycleDFS(id, visited, recStack, make([]string, 0)) {
				return fmt.Errorf("circular dependency detected involving: %s", id)
			}
		}
	}
	
	return nil
}

// detectCycleDFS DFS 检测循环
func (g *DependencyGraph) detectCycleDFS(id string, visited, recStack map[string]bool, path []string) bool {
	visited[id] = true
	recStack[id] = true
	path = append(path, id)
	
	for _, neighbor := range g.Edges[id] {
		if !visited[neighbor] {
			if g.detectCycleDFS(neighbor, visited, recStack, path) {
				return true
			}
		} else if recStack[neighbor] {
			// 发现循环
			return true
		}
	}
	
	recStack[id] = false
	return false
}

// TopologicalSort 拓扑排序
func (g *DependencyGraph) TopologicalSort() error {
	inDegree := make(map[string]int)
	
	// 初始化入度
	for id := range g.Nodes {
		inDegree[id] = 0
	}
	
	// 计算入度
	for _, neighbors := range g.Edges {
		for _, neighbor := range neighbors {
			inDegree[neighbor]++
		}
	}
	
	// 找到入度为 0 的节点
	queue := make([]string, 0)
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}
	
	// Kahn 算法
	result := make([]*DependencyNode, 0)
	
	for len(queue) > 0 {
		// 按深度排序（深度小的优先）
		sort.Slice(queue, func(i, j int) bool {
			return g.Nodes[queue[i]].Depth < g.Nodes[queue[j]].Depth
		})
		
		id := queue[0]
		queue = queue[1:]
		
		result = append(result, g.Nodes[id])
		
		for _, neighbor := range g.Edges[id] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}
	
	if len(result) != len(g.Nodes) {
		return fmt.Errorf("failed to sort dependencies: graph has cycles")
	}
	
	g.SortedNodes = result
	return nil
}

// GetConflicts 获取版本冲突
func (g *DependencyGraph) GetConflicts() []VersionConflict {
	conflicts := make([]VersionConflict, 0)
	
	// 按名称分组
	byName := make(map[string][]*DependencyNode)
	for _, node := range g.Nodes {
		byName[node.Name] = append(byName[node.Name], node)
	}
	
	// 检查每个名称的多个版本
	for name, nodes := range byName {
		if len(nodes) > 1 {
			versions := make([]string, len(nodes))
			for i, n := range nodes {
				versions[i] = n.Version
			}
			conflicts = append(conflicts, VersionConflict{
				Name:     name,
				Versions: versions,
			})
		}
	}
	
	return conflicts
}

// VersionConflict 版本冲突
type VersionConflict struct {
	Name     string
	Versions []string
}

// String 返回字符串表示
func (c VersionConflict) String() string {
	return fmt.Sprintf("%s has multiple versions: %v", c.Name, c.Versions)
}

// DependencyInjector 依赖注入器
type DependencyInjector struct {
	loader Loader
}

// NewDependencyInjector 创建依赖注入器
func NewDependencyInjector(loader Loader) *DependencyInjector {
	return &DependencyInjector{loader: loader}
}

// Inject 注入依赖
func (i *DependencyInjector) Inject(pluginID string, config map[string]interface{}) (map[string]interface{}, error) {
	// 获取插件
	managed, err := i.loader.Get(pluginID)
	if err != nil {
		return nil, err
	}
	
	// 注入配置
	result := make(map[string]interface{})
	
	// 复制原始配置
	for k, v := range config {
		result[k] = v
	}
	
	// 注入依赖
	for _, dep := range managed.Manifest.Dependencies {
		depPlugin, err := i.loader.Get(dep.ID)
		if err != nil {
			if dep.Optional {
				continue
			}
			return nil, fmt.Errorf("required dependency not loaded: %s", dep.ID)
		}
		
		// 将依赖作为服务注入
		result[dep.ID] = depPlugin.Instance
	}
	
	return result, nil
}

// ServiceLocator 服务定位器
type ServiceLocator struct {
	loader Loader
	services map[string]interface{}
	mu       sync.RWMutex
}

// NewServiceLocator 创建服务定位器
func NewServiceLocator(loader Loader) *ServiceLocator {
	return &ServiceLocator{
		loader:   loader,
		services: make(map[string]interface{}),
	}
}

// Register 注册服务
func (s *ServiceLocator)	Register(name string, service interface{}) {
	s.mu.Lock()
	s.services[name] = service
	s.mu.Unlock()
}

// Get 获取服务
func (s *ServiceLocator) Get(name string) (interface{}, error) {
	s.mu.RLock()
	service, ok := s.services[name]
	s.mu.RUnlock()
	
	if !ok {
		return nil, fmt.Errorf("service not found: %s", name)
	}
	
	return service, nil
}

// GetPlugin 获取插件作为服务
func (s *ServiceLocator) GetPlugin(pluginID string) (pipeline.NodePlugin, error) {
	managed, err := s.loader.Get(pluginID)
	if err != nil {
		return nil, err
	}
	
	if managed.Instance == nil {
		return nil, fmt.Errorf("plugin not started: %s", pluginID)
	}
	
	return managed.Instance, nil
}

// ListServices 列出所有服务
func (s *ServiceLocator) ListServices() []string {
	s.mu.RLock()
	result := make([]string, 0, len(s.services))
	for name := range s.services {
		result = append(result, name)
	}
	s.mu.RUnlock()
	
	return result
}
