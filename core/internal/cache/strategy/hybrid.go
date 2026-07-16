package strategy

import (
    "context"
    "fmt"
    "sync"
)

// HybridStrategy 混合策略: 精确和语义并行查询，先命中者返回
type HybridStrategy struct {
    exact    *ExactStrategy
    semantic *SemanticStrategy
}

// NewHybridStrategy 创建混合策略
func NewHybridStrategy(exact *ExactStrategy, semantic *SemanticStrategy) *HybridStrategy {
    return &HybridStrategy{
        exact:    exact,
        semantic: semantic,
    }
}

func (s *HybridStrategy) Name() string {
    return "hybrid"
}

func (s *HybridStrategy) SupportsSemantic() bool {
    return true
}

func (s *HybridStrategy) SetExactStrategy(exact *ExactStrategy) {
    s.exact = exact
}

func (s *HybridStrategy) SetSemanticStrategy(semantic *SemanticStrategy) {
    s.semantic = semantic
}

func (s *HybridStrategy) Configure(config map[string]interface{}) error {
    // 配置会分别传递给 exact 和 semantic 策略
    return nil
}

// Read 并行执行精确和语义查询，先命中者返回
func (s *HybridStrategy) Read(ctx context.Context, query string, opts ReadOptions) (*Result, error) {
    // 如果只有一个策略可用，直接调用
    if s.exact != nil && s.semantic == nil {
        r, err := s.exact.Read(ctx, query, opts)
        if err == nil {
            r.SourceStrategy = "exact"
        }
        return r, err
    }
    if s.semantic != nil && s.exact == nil {
        r, err := s.semantic.Read(ctx, query, opts)
        if err == nil {
            r.SourceStrategy = "semantic"
        }
        return r, err
    }

    // 两者都可用的并行查询：先命中者胜
    type readResult struct {
        r   *Result
        err error
    }

    exactCh := make(chan readResult, 1)
    semanticCh := make(chan readResult, 1)

    go func() {
        r, err := s.exact.Read(ctx, query, opts)
        if err == nil && r.Hit {
            r.SourceStrategy = "exact"
        }
        exactCh <- readResult{r, err}
    }()

    go func() {
        r, err := s.semantic.Read(ctx, query, opts)
        if err == nil && r.Hit {
            r.SourceStrategy = "semantic"
        }
        semanticCh <- readResult{r, err}
    }()

    // 收集两个结果，返回先命中的
    var exactRes, semanticRes readResult
    var gotExact, gotSemantic bool
    for !gotExact || !gotSemantic {
        select {
        case r := <-exactCh:
            exactRes, gotExact = r, true
            // 如果精确命中了，立即返回
            if r.err == nil && r.r.Hit {
                return r.r, nil
            }
        case r := <-semanticCh:
            semanticRes, gotSemantic = r, true
            // 如果语义命中了，立即返回
            if r.err == nil && r.r.Hit {
                return r.r, nil
            }
        case <-ctx.Done():
            // 上下文取消，返回已收到的最好结果
            if gotExact && exactRes.err == nil && exactRes.r.Hit {
                return exactRes.r, nil
            }
            if gotSemantic && semanticRes.err == nil && semanticRes.r.Hit {
                return semanticRes.r, nil
            }
            return &Result{Hit: false}, ctx.Err()
        }
    }

    // 都未命中，返回未命中结果
    // 优先返回 exact 的结果（可能有更多信息）
    if exactRes.err == nil {
        return exactRes.r, nil
    }
    if semanticRes.err == nil {
        return semanticRes.r, nil
    }
    // 两者都出错，返回第一个错误
    return &Result{Hit: false}, exactRes.err
}

// Write 并行写入精确和语义存储
func (s *HybridStrategy) Write(ctx context.Context, entry *Entry, opts WriteOptions) error {
    if s.exact == nil && s.semantic == nil {
        return nil
    }
    if s.exact != nil && s.semantic == nil {
        return s.exact.Write(ctx, entry, opts)
    }
    if s.semantic != nil && s.exact == nil {
        return s.semantic.Write(ctx, entry, opts)
    }

    // 并行写入
    var wg sync.WaitGroup
    var mu sync.Mutex
    var errs []error

    wg.Add(2)

    go func() {
        defer wg.Done()
        if err := s.exact.Write(ctx, entry, opts); err != nil {
            mu.Lock()
            errs = append(errs, fmt.Errorf("exact write failed: %w", err))
            mu.Unlock()
        }
    }()

    go func() {
        defer wg.Done()
        if err := s.semantic.Write(ctx, entry, opts); err != nil {
            mu.Lock()
            errs = append(errs, fmt.Errorf("semantic write failed: %w", err))
            mu.Unlock()
        }
    }()

    wg.Wait()

    if len(errs) > 0 {
        if len(errs) == 1 {
            return errs[0]
        }
        return fmt.Errorf("multiple write failures: %v", errs)
    }
    return nil
}

// Delete 并行删除精确和语义存储
func (s *HybridStrategy) Delete(ctx context.Context, key string) error {
    if s.exact == nil && s.semantic == nil {
        return nil
    }

    var wg sync.WaitGroup
    var mu sync.Mutex
    var errs []error

    if s.exact != nil {
        wg.Add(1)
        go func() {
            defer wg.Done()
            if err := s.exact.Delete(ctx, key); err != nil {
                mu.Lock()
                errs = append(errs, fmt.Errorf("exact delete failed: %w", err))
                mu.Unlock()
            }
        }()
    }

    if s.semantic != nil {
        wg.Add(1)
        go func() {
            defer wg.Done()
            if err := s.semantic.Delete(ctx, key); err != nil {
                mu.Lock()
                errs = append(errs, fmt.Errorf("semantic delete failed: %w", err))
                mu.Unlock()
            }
        }()
    }

    wg.Wait()

    if len(errs) > 0 {
        return errs[0]
    }
    return nil
}
