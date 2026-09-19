# 素材与技术资料

## 素材清单

`web/assets/tech-background-original.png`：本次会话前面已经生成的科技背景原始 PNG，原生尺寸 1672 × 941。不是 4K，没有通过放大伪称更高分辨率。

`web/assets/tech-background.webp`：同一背景的轻量 WebP，1672 × 941，供界面背景和演示图片输出使用。

`web/assets/sample-motion.mp4`：使用背景制作的 5 秒慢推进样片，1280 × 720，H.264，无音轨。仅检验视频播放器和界面，不是视频模型结果。

`web/assets/sample-voice.wav`：使用本地 eSpeak Mandarin 合成的固定中文 WAV。内容是“欢迎来到配置中心。这是一段用于播放器验证的固定语音样例。”音质只用于功能样例，不代表任何真实 TTS 模型。不是跟随本次输入合成。

`screenshots/*.png`：浏览器实际渲染原型的截图。包括只读主页面、对话、JSON、图片、TTS、视频、数据库、存储与移动端。

本包不分发字体文件，不复制真实厂商的 logo，不包含真实 API Key。原型中的字母标识只是分组符号。

## 背景应用

使用 CSS 最底层背景，叠加深色渐变，不降低正文对比度。原型 CSS 已提供可直接复用的变量、透明度和层次。

应用层建议保持少量动效，不把全屏视频当永久背景。视频样片是测试用素材，不是建议生产首页循环播放的视频壁纸。

## 官方技术资料

下列编号供各文档引用。查阅日期：2026-09-19。产品布局、颜色与交互取舍是本方案的设计建议；下列资料用于底层浏览器行为与安全实践，不表示这些机构认可本设计。

[1] MDN — Using readable streams：
https://developer.mozilla.org/en-US/docs/Web/API/Streams_API/Using_readable_streams

用于 Fetch 返回体的逐块消费；实现流式内容追加时参考。

[2] MDN — HTML video element：
https://developer.mozilla.org/en-US/docs/Web/HTML/Reference/Elements/video

用于内嵌视频、controls、playsinline、媒体类型与播放错误。

[3] MDN — HTML audio element：
https://developer.mozilla.org/en-US/docs/Web/HTML/Reference/Elements/audio

用于直接试听返回音频与播放控制。

[4] MDN — Autoplay guide for media and Web Audio APIs：
https://developer.mozilla.org/en-US/docs/Web/Media/Guides/Autoplay

用于解释为什么不依赖异步任务完成后自动播放带声音的媒体。

[5] OWASP — Server Side Request Forgery Prevention Cheat Sheet：
https://cheatsheetseries.owasp.org/cheatsheets/Server_Side_Request_Forgery_Prevention_Cheat_Sheet.html

用于已登记目标、白名单、DNS/重定向与网络边界，不把测试执行或媒体代理变成任意地址访问工具。

[6] OWASP — Secrets Management Cheat Sheet：
https://cheatsheetseries.owasp.org/cheatsheets/Secrets_Management_Cheat_Sheet.html

用于服务端秘密引用、最小权限、生命周期和防止秘密进入日志等原则。
