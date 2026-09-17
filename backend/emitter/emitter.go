// Package emitter 提供 backend 业务层与具体 GUI runtime 解耦的事件发射抽象。
// 桌面端注入 Wails v2 runtime.EventsEmit 适配；移动端(Wails v3/Android)注入
// app.Event.Emit 适配；测试或无 GUI 环境注入 Discard。
package emitter

// Emitter 向前端发送一个命名事件。payload 与 Wails runtime.EventsEmit 的
// 可变参数一致：无数据时不传，单数据时传一个值。
type Emitter func(event string, payload ...any)

// Discard 丢弃所有事件（no-op），供测试或无 GUI 环境使用。
var Discard Emitter = func(string, ...any) {}
