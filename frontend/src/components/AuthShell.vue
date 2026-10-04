<template>
  <main ref="shell" class="auth-shell">
    <div class="connections" :class="{ 'connections-enter': animateEntrance }" aria-hidden="true">
      <svg class="connection-art art-left" viewBox="0 0 560 600" fill="none" focusable="false">
        <circle class="color-plane plane-slate" cx="210" cy="365" r="158" />
        <circle class="color-plane plane-blue" cx="328" cy="260" r="118" />
        <circle class="orbit" cx="210" cy="365" r="158" />
        <circle class="orbit orbit-blue" cx="328" cy="260" r="118" />
        <circle class="orbit detail" cx="160" cy="226" r="82" />
        <circle class="orbit inner-orbit detail" cx="210" cy="365" r="132" />
        <!-- 两个社交圈的交叠区域，以一段珊瑚红弧线强调联系。 -->
        <path class="intersection" d="M230 211A158 158 0 0 1 361 318" />
        <g class="threads">
          <path :d="curve('left', 1, 0)" />
          <path :d="curve('left', 0, 2)" />
          <path :d="curve('left', 0, 3)" />
          <path class="detail" :d="curve('left', 1, 4)" />
        </g>
        <g v-for="(node, index) in nodes.left" :key="index" class="drag-node" :class="[node.detail ? 'detail' : '', { dragging: active?.node === node }]"
          :transform="`translate(${node.x + node.dx} ${node.y + node.dy})`"
          @pointerdown="startDrag($event, node)" @pointermove="moveDrag" @pointerup="endDrag" @pointercancel="endDrag" @lostpointercapture="endDrag">
          <title>拖动节点，松手回弹</title>
          <circle class="drag-hit" :r="hitRadius('left')" />
          <g class="node-appearance">
            <circle v-if="index === 0" class="node-halo" r="21" />
            <circle class="drag-focus" r="15" />
            <circle :class="node.style" :r="node.radius" />
          </g>
        </g>
        <path class="registration-mark detail" d="M73 128H87M80 121V135M425 474H439M432 467V481" />
      </svg>
      <svg class="connection-art art-right" viewBox="0 0 440 440" fill="none" focusable="false">
        <circle class="color-plane plane-blue" cx="282" cy="152" r="104" />
        <circle class="orbit" cx="282" cy="152" r="104" />
        <circle class="orbit" cx="202" cy="244" r="90" />
        <path class="echo-arc" d="M114 225A90 90 0 0 1 195 154" />
        <g class="threads">
          <path :d="curve('right', 1, 0)" />
          <path :d="curve('right', 0, 2)" />
        </g>
        <g v-for="(node, index) in nodes.right" :key="index" class="drag-node" :class="{ dragging: active?.node === node }"
          :transform="`translate(${node.x + node.dx} ${node.y + node.dy})`"
          @pointerdown="startDrag($event, node)" @pointermove="moveDrag" @pointerup="endDrag" @pointercancel="endDrag" @lostpointercapture="endDrag">
          <title>拖动节点，松手回弹</title>
          <circle class="drag-hit" :r="hitRadius('right')" />
          <g class="node-appearance">
            <circle class="drag-focus" r="15" />
            <circle :class="node.style" :r="node.radius" />
          </g>
        </g>
        <circle class="node red detail" cx="202" cy="334" r="4" />
      </svg>
    </div>
    <div class="auth-card">
      <slot />
    </div>
  </main>
</template>

<script>
// 登录与注册之间切换时，不重复播放装饰入场动画。
let hasEntered = false
</script>

<script setup>
import { onMounted, onUnmounted, reactive, ref, shallowRef } from 'vue'

const animateEntrance = !hasEntered
hasEntered = true
const shell = ref(null)
const active = shallowRef(null)
const widths = reactive({ left: 560, right: 440 })
const nodes = reactive({
  left: [
    { x: 268, y: 260, radius: 8, style: 'node red' },
    { x: 96, y: 365, radius: 8, style: 'ring' },
    { x: 405, y: 138, radius: 6, style: 'node blue' },
    { x: 354, y: 444, radius: 7, style: 'ring' },
    { x: 210, y: 497, radius: 4, style: 'node muted', detail: true },
  ].map(node => ({ ...node, dx: 0, dy: 0 })),
  right: [
    { x: 282, y: 152, radius: 7, style: 'node blue' },
    { x: 154, y: 280, radius: 6, style: 'ring' },
    { x: 370, y: 100, radius: 4, style: 'node muted' },
  ].map(node => ({ ...node, dx: 0, dy: 0 })),
})
const returns = new Map()
let observer
const reducedMotion = () => window.matchMedia('(prefers-reduced-motion: reduce)').matches
const hitRadius = side => 24 * (side === 'left' ? 560 : 440) / widths[side]

function curve(side, from, to) {
  const a = nodes[side][from]
  const b = nodes[side][to]
  const x1 = a.x + a.dx, y1 = a.y + a.dy
  const x2 = b.x + b.dx, y2 = b.y + b.dy
  const middle = (y1 + y2) / 2
  return `M${x1} ${y1}C${x1} ${middle} ${x2} ${middle} ${x2} ${y2}`
}

function startDrag(event, node) {
  if (active.value || !event.isPrimary || event.button !== 0) return
  const target = event.currentTarget
  const matrix = target.ownerSVGElement.getScreenCTM()
  if (!matrix) return
  event.preventDefault()
  cancelAnimationFrame(returns.get(node))
  returns.delete(node)
  const origin = new DOMPoint(event.clientX, event.clientY).matrixTransform(matrix.inverse())
  active.value = { node, target, pointerId: event.pointerId, origin, dx: node.dx, dy: node.dy }
  target.setPointerCapture(event.pointerId)
}

function moveDrag(event) {
  const drag = active.value
  if (!drag || event.pointerId !== drag.pointerId) return
  const matrix = drag.target.ownerSVGElement.getScreenCTM()
  if (!matrix) return
  const point = new DOMPoint(event.clientX, event.clientY).matrixTransform(matrix.inverse())
  let dx = drag.dx + point.x - drag.origin.x
  let dy = drag.dy + point.y - drag.origin.y
  // 最大牵引距离按屏幕像素计算，缩放后触感保持一致。
  const limit = 64 / Math.hypot(matrix.a, matrix.b)
  const distance = Math.hypot(dx, dy)
  if (distance > limit) { dx *= limit / distance; dy *= limit / distance }
  const screen = new DOMPoint(drag.node.x + dx, drag.node.y + dy).matrixTransform(matrix)
  const card = shell.value.querySelector('.auth-card').getBoundingClientRect()
  if (screen.x > card.left - 18 && screen.x < card.right + 18 && screen.y > card.top - 18 && screen.y < card.bottom + 18) return
  if (screen.x < 8 || screen.x > window.innerWidth - 8 || screen.y < 8 || screen.y > window.innerHeight - 8) return
  drag.node.dx = dx
  drag.node.dy = dy
}

function endDrag(event) {
  const drag = active.value
  if (!drag || (event && event.pointerId !== drag.pointerId)) return
  active.value = null
  if (drag.target.hasPointerCapture(drag.pointerId)) drag.target.releasePointerCapture(drag.pointerId)
  const { node } = drag
  const dx = node.dx, dy = node.dy
  const start = performance.now()
  const frame = now => {
    const t = Math.min((now - start) / 1000, 1)
    if (t === 1 || reducedMotion()) {
      node.dx = node.dy = 0
      returns.delete(node)
      return
    }
    const remaining = (1 - t) ** 3
    node.dx = dx * remaining
    node.dy = dy * remaining
    returns.set(node, requestAnimationFrame(frame))
  }
  frame(start)
}

onMounted(() => {
  // 统一按真实路径长度描线；内圈虚线保留原有节奏。
  shell.value.querySelectorAll('.orbit:not(.inner-orbit), .threads path, .intersection, .echo-arc').forEach(el => el.setAttribute('pathLength', '1'))
  observer = new ResizeObserver(() => {
    endDrag()
    for (const side of ['left', 'right']) {
      widths[side] = shell.value.querySelector(`.art-${side}`).getBoundingClientRect().width || 1
    }
  })
  observer.observe(shell.value)
})

onUnmounted(() => {
  observer?.disconnect()
  active.value = null
  for (const id of returns.values()) cancelAnimationFrame(id)
  returns.clear()
})
</script>

<style scoped>
.auth-shell {
  --auth-paper: #f5f4f1;
  --connection-line: #bcc5cb;
  --connection-red: #d77670;
  --connection-blue: #668cbb;
  --connection-muted: #95a3ae;
  --plane-blue: #8aafd4;
  --plane-slate: #aab9c5;
  --el-bg-color: var(--surface-raised);
  --el-fill-color-blank: var(--surface-raised);
  --el-text-color-primary: var(--text-primary);
  --el-text-color-regular: var(--text-secondary);
  --el-text-color-secondary: var(--text-tertiary);
  --el-border-color: var(--border-strong);
  position: relative;
  isolation: isolate;
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100dvh;
  padding: 40px 24px;
  background: var(--auth-paper);
}

.connections {
  position: absolute;
  inset: 0;
  z-index: 0;
  overflow: hidden;
  pointer-events: none;
}

.art-left { --entrance-delay: 0s; }
.art-right { --entrance-delay: 0.15s; }
.connections-enter :is(.orbit:not(.inner-orbit), .threads path, .intersection, .echo-arc) {
  stroke-dasharray: 1;
  animation: draw-connection 1.25s cubic-bezier(0.25, 0.1, 0.25, 1) var(--entrance-delay) both;
}
.connections-enter :is(.node-appearance, .registration-mark, .inner-orbit, .node.detail) {
  animation: reveal-detail 0.45s ease-out calc(var(--entrance-delay) + 0.75s) both;
}
.connections-enter .color-plane {
  animation: reveal-plane 0.45s ease-out calc(var(--entrance-delay) + 1.2s) both;
}

.drag-node { pointer-events: all; cursor: grab; touch-action: none; user-select: none; }
.drag-node.dragging { cursor: grabbing; }
.drag-hit { fill: transparent; }
.drag-focus { fill: none; stroke: var(--connection-blue); opacity: 0; }
.drag-node:hover .drag-focus, .drag-node.dragging .drag-focus { opacity: 0.6; }

.connections::before {
  content: '';
  position: absolute;
  inset: 0;
  opacity: 0.035;
  background-image: url("data:image/svg+xml,%3Csvg viewBox='0 0 180 180' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='grain'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='.8' numOctaves='3' stitchTiles='stitch'/%3E%3C/filter%3E%3Cpath fill='%23000' filter='url(%23grain)' d='M0 0h180v180H0z'/%3E%3C/svg%3E");
  background-size: 180px 180px;
}

.connection-art {
  position: absolute;
  height: auto;
}

.art-left { width: min(41vw, 620px); left: -2vw; bottom: -2%; }
.art-right { width: min(30vw, 430px); right: -1vw; top: -2%; }

.threads, .ring, .orbit, .registration-mark {
  stroke: var(--connection-line);
  stroke-width: 1.2;
}

.color-plane { opacity: 0.18; }
.plane-blue { fill: var(--plane-blue); }
.plane-slate { fill: var(--plane-slate); }
.orbit-blue { stroke: var(--connection-blue); opacity: 0.55; }
.inner-orbit { stroke-dasharray: 2 7; opacity: 0.65; }
.intersection { stroke: var(--connection-red); stroke-width: 12; opacity: 0.6; }
.echo-arc { stroke: var(--connection-blue); stroke-width: 5; opacity: 0.35; }
.ring, .node-halo { fill: var(--auth-paper); }
.node-halo { fill-opacity: 0.8; }
.node { fill: currentColor; }
.red { color: var(--connection-red); }
.blue { color: var(--connection-blue); }
.muted { color: var(--connection-muted); }

.auth-card {
  position: relative;
  z-index: 1;
  width: 400px;
  max-width: 100%;
  padding: 44px 40px 36px;
  background: var(--surface-raised);
  border: 1px solid var(--border-subtle);
  border-radius: var(--r-xl);
  box-shadow: 0 2px 4px rgba(17, 17, 26, 0.015), 0 12px 36px rgba(17, 17, 26, 0.03);
}

:global(body.dark .auth-shell) {
  --auth-paper: #101419;
  --connection-line: #3a4755;
  --connection-red: #b77978;
  --connection-blue: #7496bd;
  --connection-muted: #596678;
  --plane-blue: #5075a0;
  --plane-slate: #627384;
}

:global(body.dark .auth-card) {
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.08), 0 12px 36px rgba(0, 0, 0, 0.14);
}

@keyframes draw-connection { from { stroke-dashoffset: 1; } to { stroke-dashoffset: 0; } }
@keyframes reveal-detail { from { opacity: 0; } }
@keyframes reveal-plane { from { opacity: 0; } to { opacity: 0.18; } }

@media (max-width: 600px) {
  .auth-shell { padding: 32px 20px; }
  .auth-card { padding: 32px 24px; }
  .connection-art { opacity: 0.65; }
  .art-left { width: 300px; left: -170px; bottom: -60px; }
  .art-right { width: 240px; right: -130px; top: -60px; }
  .detail { display: none; }
}

@media (prefers-reduced-motion: reduce) {
  .connections-enter :is(.orbit, .threads path, .intersection, .echo-arc, .node-appearance, .registration-mark, .node.detail, .color-plane) {
    animation: none;
    stroke-dashoffset: 0;
  }
}
</style>
