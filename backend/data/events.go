package data

import "go-stock/backend/emitter"

// eventEmitter 是 data 层向前端推送事件的包级默认发射器。
// data 层不依赖任何 GUI runtime：桌面端在 App 启动时注入 EventsEmit 适配，
// 移动端注入 Wails v3 app.Event.Emit 适配，测试环境保持 Discard。
var eventEmitter emitter.Emitter = emitter.Discard

// SetEventEmitter 注入包级事件发射器（应在应用启动时调用一次）。
func SetEventEmitter(e emitter.Emitter) {
	if e == nil {
		e = emitter.Discard
	}
	eventEmitter = e
}

// emitEvent 通过包级发射器向前端推送事件；未注入时静默丢弃。
func emitEvent(event string, payload ...any) {
	eventEmitter(event, payload...)
}
