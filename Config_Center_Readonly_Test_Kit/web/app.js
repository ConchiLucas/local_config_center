/* Read-only configuration and multimedia preview prototype.
 * All test responses are explicitly labelled fixtures. No provider is contacted.
 * Catalogs come from read-only SQLite through RUN_DEMO.py; file:// uses static data.
 * No credentials, localStorage, external libraries, or configuration write routes.
 */
'use strict';
(() => {
  const $ = (s, root = document) => root.querySelector(s);
  const esc = (v) => String(v ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  const paths = {
    spark:'<path d="m12 3 2.8 6.2L21 12l-6.2 2.8L12 21l-2.8-6.2L3 12l6.2-2.8L12 3Z"/>',
    orbit:'<circle cx="12" cy="12" r="6"/><ellipse cx="12" cy="12" rx="11" ry="4" transform="rotate(-40 12 12)"/>',
    chat:'<path d="M5 4h14a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2h-8l-6 3v-3H5a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2Z"/><path d="M7 9h10M7 13h6"/>',
    image:'<rect x="3" y="3" width="18" height="18" rx="3"/><circle cx="8" cy="8" r="1.5"/><path d="m3 16 5-5 4 4 4-6 5 7"/>',
    audio:'<path d="M3 10v4m4-7v10m5-14v18m5-14v10m4-7v4"/>',
    video:'<rect x="3" y="5" width="13" height="14" rx="3"/><path d="m16 9 5-3v12l-5-3"/>',
    database:'<ellipse cx="12" cy="5" rx="8" ry="3"/><path d="M4 5v14c0 4 16 4 16 0V5M4 12c0 4 16 4 16 0"/>',
    layers:'<path d="m12 3 9 5-9 5-9-5 9-5Zm-9 9 9 5 9-5M3 16l9 5 9-5"/>',
    box:'<path d="m12 3 9 5v9l-9 5-9-5V8l9-5Zm0 9v10M3 8l9 4 9-4M8 5l9 5"/>',
    play:'<path d="m9 5 11 7-11 7V5Z"/>',
    lock:'<rect x="5" y="10" width="14" height="11" rx="2"/><path d="M8 10V7a4 4 0 0 1 8 0v3m-4 5v3"/>',
    lightning:'<path d="m13 2-9 12h7l-1 8 10-13h-7l0-7Z"/>',
    braces:'<path d="M8 3H6v6c0 2-3 3-3 3s3 1 3 3v6h2M16 3h2v6c0 2 3 3 3 3s-3 1-3 3v6h-2"/>',
    file:'<path d="M14 3H6a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9l-6-6Zm0 0v6h6M8 13h8M8 17h6"/>',
    arrow:'<path d="M4 12h16m-6-6 6 6-6 6"/>',
    check:'<path d="m5 12 4 4L19 6"/>',
    expand:'<path d="M8 3H3v5m13-5h5v5M3 16v5h5m13-5v5h-5"/>',
    collapse:'<path d="M3 8h5V3m13 5h-5V3M3 16h5v5m13-5h-5v5"/>',
    close:'<path d="m6 6 12 12M6 18 18 6"/>',
    shield:'<path d="m12 3 8 3v6c0 5-8 9-8 9s-8-4-8-9V6l8-3Z"/><path d="m8 12 3 3 5-6"/>'
  };
  const icon = (name) => `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.65" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${paths[name] || paths.box}</svg>`;
  const asset = (name) => window.EMBEDDED_ASSETS?.[name] || `assets/${name}`;
  const typeName = {chat:'对话',image:'图片',audio:'语音',video:'视频',database:'连接',storage:'存储',media:'媒体'};
  const typeIcon = {chat:'chat',image:'image',audio:'audio',video:'video',database:'database',storage:'box',media:'play'};
  let catalog;
  let selectedGroup;
  let selectedPage;
  let activeConfig = null;
  let activeGroup = null;
  let activePage = null;
  let returnFocus = null;
  const sessions = new Map();
  const dialog = $('#test-dialog');
  const bigImage = $('#image-dialog');
  const modes = [
    {id:'stream',label:'流式返回',hint:'逐步展示文字',delivery:'stream',format:'text'},
    {id:'json',label:'JSON 返回',hint:'结构化结果',delivery:'complete',format:'json'},
    {id:'complete',label:'一次性返回',hint:'完整内容展示',delivery:'complete',format:'text'}
  ];
  const getSession = (c) => {
    if (!sessions.has(c.id)) sessions.set(c.id, {input:'',mode:'stream',status:'idle',result:null,messages:[],token:0,jsonView:'tree'});
    return sessions.get(c.id);
  };
  function toast(text) {
    $('#toast').textContent = text; $('#toast').hidden = false;
    clearTimeout(toast.timer); toast.timer = setTimeout(() => { $('#toast').hidden = true; }, 4000);
  }
  const badge = '<div class="result-badge">本地样例 · 不是模型实测结果</div>';
  const renderWave = (cls, count=44) => `<div class="${cls}" aria-hidden="true">${Array.from({length:count},(_,i)=>`<i style="height:${10 + ((i*17 + i*i*7)%47)}px"></i>`).join('')}</div>`;
  const capabilityItem = (ic,title,desc) => `<div class="capability-item">${icon(ic)}<div><strong>${esc(title)}</strong><span>${esc(desc)}</span></div></div>`;
  function capabilityPane(c) {
    let body;
    if (c.kind === 'chat') {
      body = `<h3>返回能力 · 演示注册</h3>${capabilityItem('lightning','流式返回','生成内容逐步显示')}${capabilityItem('braces','JSON 返回','以结构化内容查看')}${capabilityItem('file','一次性返回','完成后展示完整内容')}`;
    } else if (c.kind === 'image') {
      body = '<h3>图片输出</h3><div class="ratio-preview">'+icon('image')+'<span>16 : 9</span></div>'+capabilityItem('expand','直接预览 · 点击放大','尺寸与格式以实际返回为准');
    } else if (c.kind === 'audio') {
      body = '<h3>语音输出</h3>'+renderWave('waveform',28)+'<div class="format-tags"><span>试听</span><span>暂停</span><span>进度</span></div>'+capabilityItem('audio','文字变成可试听的内容','本原型使用固定语音样例');
    } else if (c.kind === 'video') {
      body = '<h3>视频输出</h3><div class="ratio-preview">'+icon('play')+'<span>生成任务 → 直接播放</span></div>'+capabilityItem('video','封面 · 进度 · 全屏','生成状态与播放器状态分开');
    } else if (c.kind === 'database') {
      body = '<h3>只读连接探针</h3><div class="connection-track"><span>应用后端</span>'+icon('arrow')+'<span>目标服务</span></div>'+capabilityItem('shield','受控探针','不开放任意 SQL 或写入操作')+capabilityItem('database',c.probe||'SELECT 1','演示未连接目标数据库');
    } else {
      body = '<h3>对象与媒体预览</h3><div class="storage-symbol"><span>'+icon('image')+'</span><span>'+icon('audio')+'</span><span>'+icon('video')+'</span></div>'+capabilityItem('box','按文件类型直接展示','图片、音频、视频各有组件')+capabilityItem('shield','只读访问','不提供上传、删除或覆盖入口');
    }
    return `<aside class="capability-pane">${body}<div class="capability-note">注册样例仅展示界面结构，<br>真实能力由后端适配器确认。</div></aside>`;
  }
  function renderShell() {
    $('#primary-nav').innerHTML = catalog.groups.map(g => `<button data-group="${esc(g.id)}" class="${g.id===selectedGroup.id?'active':''}" ${g.id===selectedGroup.id?'aria-current="page"':''}>${icon(g.icon)}${esc(g.label)}</button>`).join('');
    $('#provider').innerHTML = `<div class="provider-mark">${esc(selectedGroup.mark)}</div><div><strong>${esc(selectedGroup.label)}</strong><small>${esc(selectedGroup.eyebrow)}</small></div>`;
    $('#secondary-nav').innerHTML = selectedGroup.pages.map(p=>`<button data-page="${esc(p.id)}" class="secondary-button ${p.id===selectedPage.id?'active':''}" ${p.id===selectedPage.id?'aria-current="page"':''}>${icon(p.icon)}<span><strong>${esc(p.label)}</strong><small>${esc(p.en)}</small></span></button>`).join('');
    $('#page-header').innerHTML = `<div class="page-heading"><div><div class="breadcrumb">配置中心<span class="sep">/</span>${esc(selectedGroup.label)}<span class="sep">/</span>${esc(selectedPage.label)}</div><h1>${esc(selectedPage.label)}</h1><p class="page-description">${esc(selectedPage.description)}</p></div><div class="page-identifier">${esc(selectedPage.en.toUpperCase())}</div></div>`;
    $('#configs').innerHTML = `<div class="config-count"><span>${selectedPage.configs.length} 项配置</span><span class="tiny-line"></span><span>数据库驱动 · 无编辑入口</span></div>` + selectedPage.configs.map(c => `
      <article class="card"><header class="card-header"><span class="card-icon">${icon(typeIcon[c.kind])}</span><div class="card-title"><h2>${esc(c.label)}</h2><small>${esc(c.model)}</small></div><span class="status-chip">未实测</span><button data-test="${esc(c.id)}" class="button primary">${icon('play')}测试${c.kind==='database'?'连接':'效果'}</button></header>
      <div class="card-body"><section class="config-data"><div class="data-label">${icon('lock')}配置详情 · 只读</div><dl class="property-grid">${c.properties.map(([k,v])=>`<div><dt>${esc(k)}</dt><dd class="${/[a-zA-Z]{3}/.test(v)?'mono':''}">${esc(v)}</dd></div>`).join('')}</dl><div class="credential"><span>认证凭据</span><strong>${esc(c.credentialState)}</strong></div></section>${capabilityPane(c)}</div>
      <footer class="card-bottom"><span>测试仅使用本次输入，不修改原始配置</span><span>${esc(c.id)} · ${esc(c.revision)}</span></footer></article>`).join('');
    if (!selectedPage.configs.length) $('#configs').innerHTML = '<div class="empty-result"><strong>暂无配置</strong><p>在数据库中登记配置后，这里会自动展示。页面不提供新增入口。</p></div>';
    $('#bottom-note').textContent = '说明：以上为演示配置，不代表厂商真实能力；所有测试结果均为本地样例，不会调用真实服务或产生模型费用。';
  }
  function route() {
    const parts = location.hash.replace(/^#\/?/,'').split('/');
    selectedGroup = catalog.groups.find(g=>g.id===parts[0]) || catalog.groups[0];
    selectedPage = selectedGroup.pages.find(p=>p.id===parts[1]) || selectedGroup.pages[0];
    renderShell();
  }
  function navigate(gid,pid) {
    const target=`#/${gid}/${pid}`;
    if (location.hash===target) route(); else location.hash=target;
  }
  function buildInput(c,s) {
    const promptLabel = c.kind==='audio'?'要朗读的文本':c.kind==='chat'?'输入你的内容':'描述想要生成的内容';
    const placeholder = {chat:'例如：用三句话解释这个配置中心的用途。',image:'例如：深海蓝色背景，细腻的青色光线，留出干净的内容区。',audio:'请输入要转换成语音的文字。',video:'例如：镜头缓慢推进，青色数据光点在深蓝空间中移动。'};
    if (['chat','image','audio','video'].includes(c.kind)) {
      const modeHtml = c.kind==='chat' ? `<label class="test-field-label">本次测试的返回方式</label><div class="mode-pills">${modes.map(m=>`<button class="mode-pill ${s.mode===m.id?'active':''}" data-mode="${m.id}" aria-pressed="${s.mode===m.id}"><span>${esc(m.label)}</span><small>${esc(m.hint)}</small></button>`).join('')}</div>` : '';
      $('#input-fields').innerHTML = modeHtml+`<label class="test-field-label" for="prompt">${promptLabel}</label><textarea id="prompt" class="textarea" maxlength="4000" placeholder="${esc(placeholder[c.kind])}">${esc(s.input)}</textarea><p class="helper-text">仅用于本次测试，不会写回配置。<br>演示输出与真实服务尚未连接。</p><div class="fixed-spec"><strong>当前配置</strong>${esc(c.model)}<br>${c.kind==='chat'?'正式接入后按服务端能力返回。':c.kind==='audio'?'音色与格式沿用数据库默认值。':c.kind==='video'?'时长与画幅沿用数据库默认值。':'尺寸与格式沿用数据库默认值。'}</div>`;
    } else if (c.kind==='database') {
      $('#input-fields').innerHTML = `<label class="test-field-label">测试范围</label><div class="fixed-spec"><strong>预设的只读探针</strong>${esc(c.probe||'SELECT 1')}<br>不执行任意输入的命令</div><p class="helper-text">正式接入时由服务端执行白名单探针，返回连接状态与耗时。演示不会连接目标服务。</p><div class="fixed-spec"><strong>目标配置</strong>${esc(c.label)}<br>${esc(c.model)}</div>`;
    } else {
      $('#input-fields').innerHTML = `<label class="test-field-label" for="prompt">对象名称或前缀</label><textarea id="prompt" class="textarea" maxlength="4000" placeholder="例如：demo/">${esc(s.input)}</textarea><p class="helper-text">只读列举或预览。演示只展示包内的固定图片、语音和视频，不访问真实存储桶。</p><div class="fixed-spec"><strong>媒体展示方式</strong>图片：缩略图与大图<br>视频：内嵌播放器<br>语音：内嵌音频播放器</div>`;
    }
    const disclaimers={chat:'这里只演示返回过程与结果排版，不代表模型真实回答。',image:'当前展示固定背景素材，不会根据输入生成新图片。',audio:'当前播放固定语音样例，不会按输入内容合成新语音。',video:'当前播放本地动效样片，不会根据输入生成新视频。',database:'演示不连接真实数据库，不会显示伪造的连接成功。',storage:'演示使用本地媒体，不读取真实 MinIO。',media:'演示使用本地媒体，不读取真实 MinIO。'};
    $('#test-disclaimer').textContent=disclaimers[c.kind];
    $('#run-test').innerHTML = icon('play')+(c.kind==='database'?'运行探针演示':c.kind==='storage'||c.kind==='media'?'预览样例对象':'开始测试演示');
    const prompt=$('#prompt');
    if(prompt) prompt.addEventListener('input',()=>{s.input=prompt.value;});
  }
  function openTest(id,trigger) {
    activeConfig = selectedPage.configs.find(c=>c.id===id);
    if (!activeConfig) return;
    activeGroup=selectedGroup; activePage=selectedPage;
    returnFocus=trigger;
    const c=activeConfig,s=getSession(c);
    $('#test-title').textContent = `${typeName[c.kind]}测试`;
    $('#test-icon').innerHTML = icon(typeIcon[c.kind]);
    $('#test-context').innerHTML = `<span>${esc(activeGroup.label)}</span><span class="divider">/</span><span>${esc(c.label)}</span><span class="divider">·</span><span>${esc(c.model)}</span><span class="context-tail">绑定版本 ${esc(c.revision)} · 不改变数据库配置</span>`;
    buildInput(c,s);renderResult();
    if(!dialog.open)dialog.showModal();
    setTimeout(()=>($('#prompt')||$('#run-test')).focus(),100);
  }
  function pauseMedia(root=document) {root.querySelectorAll('audio,video').forEach(el=>el.pause());}
  function closeTest() { pauseMedia(dialog);dialog.close();if(returnFocus?.isConnected)returnFocus.focus(); }
  function syncRunning() {
    const s=getSession(activeConfig),running=s.status==='running';
    $('#run-test').hidden=running;$('#stop-test').hidden=!running;
    $('#input-fields').querySelectorAll('textarea,button').forEach(el=>el.disabled=running);
    $('#run-status').textContent=({idle:'等待输入',running:'演示运行中',done:'样例已展示',error:'演示失败',cancelled:'演示已停止'})[s.status] || '等待输入';
  }
  function emptyHtml(c) {
    const hints={chat:'输入内容后，回答会出现在这里。\n流式、JSON 与一次性返回使用不同的结果排版。',image:'返回的图片直接展示。\n点击图片即可放大，不需要复制地址。',audio:'文字输入完成后，在这里点击试听。\n支持播放、暂停与拖动进度。',video:'生成完成后，在这里直接播放。\n排队、生成与播放状态分别展示。',database:'运行只读探针后，显示状态、耗时与受控结果。\n本原型只演示排版，不会连接目标服务。',storage:'读取对象后，按媒体类型直接预览。\n不需要离开当前工作台。',media:'图片、音频与视频都在这里预览。\n当前使用包内固定样例。'};
    return `<div class="empty-result"><span class="empty-mark">${icon(typeIcon[c.kind])}</span><strong>让结果直接可见</strong><p>${esc(hints[c.kind]).replace(/\n/g,'<br>')}</p></div>`;
  }
  function treeNode(value,key='result') {
    if (value!==null && typeof value==='object') return `<details open><summary>${esc(key)} <span style="color:var(--quiet)">${Array.isArray(value)?`[${value.length}]`:'{ }'}</span></summary><div style="margin-left:12px">${Object.entries(value).map(([k,v])=>treeNode(v,k)).join('')}</div></details>`;
    return `<div class="leaf"><b>${esc(key)}:</b>${esc(JSON.stringify(value))}</div>`;
  }
  function resultChat(c,s) {
    if(s.result?.kind==='json') {
      return badge+`<h3 class="result-title">结构化返回</h3><div class="json-switch"><button data-json="tree" class="${s.jsonView==='tree'?'active':''}">结构视图</button><button data-json="raw" class="${s.jsonView==='raw'?'active':''}">格式化源码</button></div>${s.jsonView==='tree'?`<div class="json-tree">${treeNode(s.result.value)}</div>`:`<pre class="json-box">${esc(JSON.stringify(s.result.value,null,2))}</pre>`}<p class="result-note">这是为演示构造的 JSON，不代表真实模型遵守了任何输出约束。</p>`;
    }
    return badge+s.messages.map(m=>`<div class="message ${m.role}"><span class="message-label">${m.role==='user'?'你的输入':'演示回答 · 非模型输出'}</span><span class="${m.streaming?'stream-cursor':''}">${esc(m.text)}</span></div>`).join('');
  }
  function resultImage(){return badge+`<h3 class="result-title">图片结果</h3><button class="result-image" data-enlarge="true" aria-label="放大图片结果"><img src="${asset('tech-background.webp')}" alt="固定样例：深蓝青色科技背景；不是根据本次提示词生成的图片" /></button><div class="asset-meta"><strong>tech-background.webp</strong><span>1672 × 941 · 点击图片放大</span></div><p class="result-note">本原型直接预览已提供的背景素材。正式接入后，此位置显示真实生成的图片。</p>`;}
  function resultAudio(){return badge+`<div class="audio-hero"><div class="audio-title">${icon('audio')}语音已就绪 · 固定样例</div><div class="audio-sub">本地 WAV 文件 · 仅验证播放器体验</div>${renderWave('waveform')}<audio controls preload="metadata" src="${asset('sample-voice.wav')}" aria-label="播放固定语音样例"></audio></div><div class="transcript"><span>这段样例实际朗读的内容</span>欢迎来到配置中心。这是一段用于播放器验证的固定语音样例。</div><p class="result-note">上方波形为装饰，不是音频分析结果。该录音与本次输入无关；生产版本显示真正合成的语音。</p>`;}
  function resultVideo(){return badge+`<h3 class="result-title">视频结果</h3><video class="sample-video" controls playsinline preload="metadata" poster="${asset('tech-background.webp')}" src="${asset('sample-motion.mp4')}" aria-label="播放固定科技背景动效样片"></video><div class="asset-meta"><strong>sample-motion.mp4</strong><span>5 秒 · 1280 × 720 · 无音轨</span></div><p class="result-note">这是由现有背景制作的缓慢推进样片，不是视频模型生成结果。可直接播放、拖动进度或全屏查看。</p>`;}
  function resultDatabase(c){return badge+`<h3 class="result-title">连接检查 · 展示样例</h3><table class="result-table"><thead><tr><th scope="col">检查项</th><th scope="col">当前结果</th></tr></thead><tbody><tr><td>目标服务</td><td>${esc(c.model)}</td></tr><tr><td>真实连接状态</td><td>未执行 · 需要后端适配器</td></tr><tr><td>预设探针</td><td>${esc(c.probe||'SELECT 1')}</td></tr><tr><td>真实耗时</td><td>未测量</td></tr><tr><td>写入操作</td><td>不开放</td></tr></tbody></table><p class="result-note">正式接入后这里显示真实探针结果；未执行连接时不能标成“连接成功”。</p>`;}
  function resultStorage(){return badge+`<h3 class="result-title">对象预览 · 3 个本地样例</h3><div class="asset-list"><div class="asset-row"><div class="asset-name">${icon('image')}demo/tech-background.webp</div><button class="result-image" data-enlarge="true" aria-label="放大存储图片样例"><img src="${asset('tech-background.webp')}" alt="固定背景样例" /></button></div><div class="asset-row"><div class="asset-name">${icon('audio')}demo/sample-voice.wav</div><audio controls preload="metadata" src="${asset('sample-voice.wav')}" aria-label="播放存储语音样例"></audio></div><div class="asset-row"><div class="asset-name">${icon('video')}demo/sample-motion.mp4</div><video class="sample-video" controls playsinline preload="metadata" poster="${asset('tech-background.webp')}" src="${asset('sample-motion.mp4')}" aria-label="播放存储视频样例"></video></div></div><p class="result-note">当前前缀不用于真实检索；对象全部来自本地素材包，未连接 MinIO。</p>`;}
  function renderResult() {
    if(!activeConfig)return;
    const c=activeConfig,s=getSession(c),target=$('#result');
    syncRunning();
    if(s.status==='idle'){target.innerHTML=emptyHtml(c);return;}
    if(s.status==='error'){target.innerHTML='<div class="empty-result"><strong>本次测试演示失败</strong><p>这是输入 #error 触发的错误状态样例。输入内容已保留，可修改后重新测试。真实接口接入后显示脱敏错误与请求编号。</p></div>';return;}
    if(s.status==='cancelled'){target.innerHTML='<div class="empty-result"><strong>演示已停止</strong><p>本次输入仍然保留。生产环境必须区分停止等待与上游任务真正取消。</p></div>';return;}
    if(c.kind==='chat' && (s.messages.length || s.result?.kind==='json')) {target.innerHTML=resultChat(c,s);if(s.status==='running')target.scrollTop=target.scrollHeight;return;}
    if(s.status==='running'){target.innerHTML=`<div class="loader-wrap"><span class="spinner" aria-hidden="true"></span>${c.kind==='video'?'演示任务：等待样片就绪':'正在展示本地样例'}<p>不是上游真实生成进度，也没有使用虚构百分比。</p></div>`;return;}
    const renderer={image:resultImage,audio:resultAudio,video:resultVideo,database:()=>resultDatabase(c),storage:resultStorage,media:resultStorage};
    target.innerHTML=(renderer[c.kind]||(()=>emptyHtml(c)))();
    target.querySelectorAll('audio,video,img').forEach(el=>el.addEventListener('error',()=>{
      if(el.dataset.errorShown)return;el.dataset.errorShown='1';
      const p=document.createElement('p');p.className='result-note';p.textContent='该媒体加载失败。请使用包内启动脚本打开，并确认素材文件存在、浏览器支持此格式。';el.insertAdjacentElement('afterend',p);
    }));
  }
  const delay=ms=>new Promise(resolve=>setTimeout(resolve,ms));
  async function runTest() {
    if(!activeConfig)return;
    const c=activeConfig,s=getSession(c),input=$('#prompt');
    if(input)s.input=input.value;
    if(['chat','image','audio','video'].includes(c.kind)&&!s.input.trim()){toast('请先输入本次测试的内容。');input?.focus();return;}
    if(s.status==='running')return;
    const token=++s.token;
    pauseMedia(dialog);s.status='running';s.result=null;
    if(c.kind==='chat'&&s.mode==='json')s.messages=[];
    if(c.kind==='chat'&&s.mode!=='json')s.messages.push({role:'user',text:s.input});
    renderResult();
    const update=()=>{if(activeConfig?.id===c.id)renderResult();};
    await delay(c.kind==='video'?1500:650);
    if(s.token!==token)return;
    if(s.input.trim()==='#error'){s.status='error';update();return;}
    if(c.kind==='chat'){
      if(s.mode==='json'){
        s.result={kind:'json',value:{demo:true,source:'local-fixture',input:s.input,configId:c.id,response:{message:'这是结构化返回的展示样例。',delivery:'complete',format:'json',providerCalled:false}}};
      }else{
        const text=`已收到你的输入：“${s.input}”\n\n这是本地构造的演示回答，并非 ${activeGroup?.label||'供应商'} 的真实模型输出。\n\n正式接入后，这里将显示后端实际返回的内容；流式内容逐步追加，完整返回一次展示。模型配置只从数据库读取，不会因为这次测试而改变。`;
        const msg={role:'assistant',text:'',streaming:s.mode==='stream'};s.messages.push(msg);
        if(s.mode==='stream'){
          const chars=Array.from(text);
          for(let i=0;i<chars.length;i+=5){
            if(s.token!==token){msg.streaming=false;return;}
            msg.text=chars.slice(0,i+5).join('');update();await delay(32);
          }
        }else msg.text=text;
        msg.streaming=false;
      }
    }else s.result={kind:c.kind,fixture:true};
    s.status='done';update();
  }
  function stopTest(){if(!activeConfig)return;const s=getSession(activeConfig);s.token++;s.status='cancelled';s.messages.forEach(m=>m.streaming=false);renderResult();}
  document.addEventListener('click',event=>{
    const el=event.target.closest('button');if(!el)return;
    if(el.dataset.group){const g=catalog.groups.find(g=>g.id===el.dataset.group);navigate(g.id,g.pages[0]?.id||'');}
    if(el.dataset.page)navigate(selectedGroup.id,el.dataset.page);
    if(el.dataset.test)openTest(el.dataset.test,el);
    if(el.dataset.mode&&activeConfig){const s=getSession(activeConfig);if(s.status==='running')return;s.mode=el.dataset.mode;buildInput(activeConfig,s);}
    if(el.dataset.json&&activeConfig){getSession(activeConfig).jsonView=el.dataset.json;renderResult();}
    if(el.dataset.enlarge){$('#large-image').src=asset('tech-background.webp');bigImage.showModal();}
  });
  $('#run-test').addEventListener('click',runTest);
  $('#stop-test').addEventListener('click',stopTest);
  $('#close-test').innerHTML=icon('close');$('#close-test').addEventListener('click',closeTest);
  $('#expand-test').innerHTML=icon('expand');$('#expand-test').addEventListener('click',()=>{
    const full=dialog.classList.toggle('full');$('#expand-test').innerHTML=icon(full?'collapse':'expand');$('#expand-test').setAttribute('aria-label',full?'还原侧栏尺寸':'展开为全屏');$('#expand-test').title=full?'还原侧栏尺寸':'展开为全屏';
  });
  $('#close-image').innerHTML=icon('close');$('#close-image').addEventListener('click',()=>bigImage.close());
  dialog.addEventListener('close',()=>pauseMedia(dialog));
  document.addEventListener('keydown',event=>{
    if(dialog.open&&!bigImage.open&&(event.ctrlKey||event.metaKey)&&event.key==='Enter'){event.preventDefault();runTest();}
  });
  window.addEventListener('hashchange',()=>{if(catalog)route();});
  async function init(){
    catalog=window.DEMO_CATALOG;
    if(location.protocol!=='file:'){
      try{
        const controller=new AbortController();const timer=setTimeout(()=>controller.abort(),3500);
        let res;try{res=await fetch('/api/catalog',{signal:controller.signal,cache:'no-store'});}finally{clearTimeout(timer);}
        if(!res.ok)throw new Error('Catalog API not connected');
        const remote=await res.json();if(!Array.isArray(remote.groups))throw new Error('Invalid catalog');catalog=remote;
      }catch(error){console.info('Prototype only: using explicitly labelled static fixture.',error.message);}
    }
    $('#source-label').innerHTML=`<span class="dot amber"></span>${catalog.source==='sqlite-demo'?'SQLite 演示库':'静态样例'} · 未接真实服务`;
    if(!catalog.groups.length){$('#main').textContent='数据库中没有配置分组。请通过外部管理流程登记后刷新。';return;}
    route();document.documentElement.dataset.ready='true';
  }
  init().catch(error=>{console.error(error);$('#main').textContent='原型初始化失败，请使用 RUN_DEMO.py 启动并检查演示数据库。';});
})();
