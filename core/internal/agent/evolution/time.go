package evolution

import "time"

// timeNow 可测试的时钟入口（默认 UTC）。
var timeNow = func() time.Time { return time.Now().UTC() }
