/* Read-only configuration catalog + live multimedia test workbench.
 * Catalog and test results come from this app's backend. Demo fixtures are not used as success output.
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
    expand:'<path d="M8 3H3v5m13-5h5v5M3 16v5h5m13-5v5h-5"/>',
    collapse:'<path d="M3 8h5V3m13 5h-5V3M3 16h5v5m13-5h-5v5"/>',
    close:'<path d="m6 6 12 12M6 18 18 6"/>',
    shield:'<path d="m12 3 8 3v6c0 5-8 9-8 9s-8-4-8-9V6l8-3Z"/><path d="m8 12 3 3 5-6"/>'
  };
  const icon = (name) => `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.65" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${paths[name] || paths.box}</svg>`;
  const typeName = {chat:'对话',image:'图片',audio:'语音',transcribe:'听写',video:'视频',database:'连接',storage:'存储',media:'媒体',cli:'CLI'};
  const typeIcon = {chat:'chat',image:'image',audio:'audio',transcribe:'audio',video:'video',database:'database',storage:'box',media:'play',cli:'layers'};
  const modes = [
    {id:'stream',label:'流式返回',hint:'逐步展示文字',delivery:'stream',format:'text'},
    {id:'json',label:'JSON 返回',hint:'结构化结果',delivery:'complete',format:'json'},
    {id:'complete',label:'一次性返回',hint:'完整内容展示',delivery:'complete',format:'text'}
  ];
  const credLabel = {configured:'已配置',missing:'未配置',not_required:'不适用'};
  const statusLabel = {unverified:'未实测',verified:'最近验证通过',stale:'旧版本结果',failed:'最近失败'};

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

  const getSession = (c) => {
    if (!sessions.has(c.id)) {
      sessions.set(c.id, {
        input: '', mode: 'stream', status: 'idle', run: null, outputs: [],
        messages: [], jsonView: 'tree', error: null, abort: null, runId: null,
        model: c.model || '', audioBase64: '', filename: '',
        family: '', resolution: '', aspect: ''
      });
      const parsed = parseAntigravityImage(c.model);
      const s = sessions.get(c.id);
      s.family = parsed.family;
      s.resolution = parsed.resolution;
      s.aspect = parsed.aspect;
    }
    return sessions.get(c.id);
  };

  function findConfigById(id) {
    if (!catalog) return null;
    for (const group of catalog.groups) {
      for (const page of group.pages) {
        const found = page.configs.find(item => item.id === id);
        if (found) return found;
      }
    }
    return null;
  }

  function usesModelCombo(c) {
    return (c.models && c.models.length) || c.allowCustomModel;
  }

  function usesImagePicker(c) {
    return c.modelPicker && c.modelPicker.style === 'antigravity-image';
  }

  function parseAntigravityImage(slug) {
    const value = String(slug || '');
    if (!value || value === 'gemini-3.1-flash-image' || value.startsWith('gemini-3.1-flash-image')) {
      return {family: 'flash-3.1', resolution: '', aspect: ''};
    }
    const prefix = 'gemini-3-pro-image';
    if (!value.startsWith(prefix)) {
      return {family: 'flash-3.1', resolution: '', aspect: ''};
    }
    let rest = value.slice(prefix.length).replace(/^-/, '');
    let resolution = '';
    if (rest === '2k' || rest.startsWith('2k-')) {
      resolution = '2k';
      rest = rest === '2k' ? '' : rest.slice(3);
    } else if (rest === '4k' || rest.startsWith('4k-')) {
      resolution = '4k';
      rest = rest === '4k' ? '' : rest.slice(3);
    }
    return {family: 'pro-3', resolution, aspect: rest};
  }

  function composeAntigravityImage(family, resolution, aspect) {
    if (family !== 'pro-3') return 'gemini-3.1-flash-image';
    let slug = 'gemini-3-pro-image';
    if (resolution) slug += '-' + resolution;
    if (aspect) slug += '-' + aspect;
    return slug;
  }

  function pickerHTML(c, s) {
    const picker = c.modelPicker;
    if (!picker || picker.style !== 'antigravity-image') return '';
    const parsed = parseAntigravityImage(s.model || c.model);
    const family = s.family || parsed.family;
    const resolution = s.resolution != null ? s.resolution : parsed.resolution;
    const aspect = s.aspect != null ? s.aspect : parsed.aspect;
    const families = picker.families || [];
    const pro = families.find(item => item.id === 'pro-3') || {resolutions: [], aspects: []};
    const isPro = family === 'pro-3';
    const familyOpts = families.map(item => `<option value="${esc(item.id)}" ${item.id===family?'selected':''}>${esc(item.label)}</option>`).join('');
    const resOpts = (pro.resolutions || []).map(item => `<option value="${esc(item.id)}" ${item.id===resolution?'selected':''}>${esc(item.label)}</option>`).join('');
    const aspOpts = (pro.aspects || []).map(item => `<option value="${esc(item.id)}" ${item.id===aspect?'selected':''}>${esc(item.label)}</option>`).join('');
    const slug = composeAntigravityImage(family, resolution, aspect);
    return `<div class="param-grid" data-picker="antigravity-image" data-config="${esc(c.id)}"><label>底模<select data-param="family">${familyOpts}</select></label><label class="param-pro" ${isPro?'':'hidden'}>分辨率<select data-param="resolution">${resOpts}</select></label><label class="param-pro" ${isPro?'':'hidden'}>画幅<select data-param="aspect">${aspOpts}</select></label></div><p class="combo-hint">实际调用 <code class="composed-model" data-config="${esc(c.id)}">${esc(slug)}</code>。Flash 无后缀；Pro 用分辨率和画幅拼模型名。</p>`;
  }

  function comboHTML(c, value) {
    const models = c.models || [];
    const current = value || c.model || '';
    let lastCat = null;
    const parts = [];
    models.forEach(m => {
      const cat = m.category || '';
      if (cat && cat !== lastCat) {
        parts.push(`<div class="combo-group" data-group="${esc(cat)}">${esc(cat)}</div>`);
        lastCat = cat;
      }
      const bits = [];
      if (m.note) bits.push(esc(m.note));
      if (m.registered) bits.push('已登记');
      const extra = bits.length ? `<small>${bits.join(' · ')}</small>` : '';
      parts.push(`<button type="button" class="combo-option" data-config="${esc(c.id)}" data-model="${esc(m.model)}" data-group="${esc(cat)}"><span>${esc(m.model)}</span>${extra}</button>`);
    });
    return `<div class="combo" data-config="${esc(c.id)}"><div class="combo-shell"><div class="combo-row"><input class="combo-input" data-config="${esc(c.id)}" value="${esc(current)}" placeholder="选择模型，或手输模型名" autocomplete="off" spellcheck="false" /><button type="button" class="combo-toggle" data-combo-toggle="${esc(c.id)}" aria-label="打开模型列表">▾</button></div><div class="combo-menu" hidden>${parts.join('') || '<div class="combo-empty">没有预置项，可直接手输</div>'}</div></div><p class="combo-hint">下拉按类别列出可用模型，也可直接手输。测试只改本次调用，不写回配置。</p></div>`;
  }

  function closeCombos() {
    document.querySelectorAll('.combo-menu').forEach(menu => { menu.hidden = true; });
  }

  function applyModel(configId, model) {
    const cfg = findConfigById(configId);
    if (!cfg) return;
    const s = getSession(cfg);
    s.model = model;
    const parsed = parseAntigravityImage(model);
    if (usesImagePicker(cfg) && (model.startsWith('gemini-3.1-flash-image') || model.startsWith('gemini-3-pro-image'))) {
      s.family = parsed.family;
      s.resolution = parsed.resolution;
      s.aspect = parsed.aspect;
    }
    document.querySelectorAll('.combo-input').forEach(input => { if (input.dataset.config === configId) input.value = model; });
    document.querySelectorAll('.composed-model').forEach(el => { if (el.dataset.config === configId) el.textContent = model; });
    document.querySelectorAll('.param-grid').forEach(grid => {
      if (grid.dataset.config !== configId) return;
      const family = grid.querySelector('[data-param="family"]');
      const resolution = grid.querySelector('[data-param="resolution"]');
      const aspect = grid.querySelector('[data-param="aspect"]');
      if (family && s.family) family.value = s.family;
      if (resolution) resolution.value = s.resolution || '';
      if (aspect) aspect.value = s.aspect || '';
      grid.querySelectorAll('.param-pro').forEach(el => { el.hidden = s.family !== 'pro-3'; });
    });
    if (activeConfig && activeConfig.id === configId) {
      const c = activeConfig, s = getSession(c);
      $('#test-context').innerHTML = `<span>${esc(activeGroup.label)}</span><span class="divider">/</span><span>${esc(c.title||c.label)}</span><span class="divider">·</span><span>${esc(s.model || c.model)}</span><span class="context-tail">绑定版本 ${esc(c.revision)} · 不改变数据库配置</span>`;
    }
  }

  function filterCombo(combo, query) {
    if (!combo) return;
    const q = String(query || '').toLowerCase();
    combo.querySelectorAll('.combo-option').forEach(option => {
      const text = option.textContent.toLowerCase();
      option.hidden = q !== '' && !text.includes(q) && !(option.dataset.model || '').toLowerCase().includes(q);
    });
    combo.querySelectorAll('.combo-group').forEach(group => {
      const cat = group.dataset.group || '';
      const any = [...combo.querySelectorAll('.combo-option')].some(option => !option.hidden && (option.dataset.group || '') === cat);
      group.hidden = !any;
    });
  }

  function toast(text) {
    $('#toast').textContent = text;
    $('#toast').hidden = false;
    clearTimeout(toast.timer);
    toast.timer = setTimeout(() => { $('#toast').hidden = true; }, 4000);
  }

  function propertiesOf(c) {
    if (Array.isArray(c.properties) && c.properties.length && Array.isArray(c.properties[0])) return c.properties;
    if (Array.isArray(c.publicProperties)) return c.publicProperties.map(p => [p.label, p.value]);
    return [];
  }

  function capabilityPane(c) {
    const caps = c.capabilities || {};
    let body;
    if (c.kind === 'chat') {
      const stream = (caps.delivery || []).includes('stream');
      const json = (caps.formats || []).includes('json');
      body = `<h3>返回能力</h3>${capabilityItem('lightning','流式返回', stream ? '后端按增量事件追加' : '当前配置未声明流式')}
        ${capabilityItem('braces','JSON 返回', json ? '完成后按 JSON 校验展示' : '未声明 JSON 约束')}
        ${capabilityItem('file','一次性返回','完成后展示完整内容')}`;
    } else if (c.kind === 'image') {
      body = '<h3>图片输出</h3><div class="ratio-preview">'+icon('image')+'<span>尺寸以实际返回为准</span></div>'+capabilityItem('expand','直接预览 · 点击放大', caps.referenceImage ? '该配置声明支持参考图；当前工作台使用文本提示' : '文本提示生成');
    } else if (c.kind === 'audio') {
      body = '<h3>语音输出</h3><div class="format-tags"><span>试听</span><span>暂停</span><span>进度</span></div>'+capabilityItem('audio','文字变成可试听的内容','播放器加载本次合成的音频，不是样例录音');
    } else if (c.kind === 'transcribe') {
      body = '<h3>听写输出</h3>'+capabilityItem('file','上传音频，返回文字','调用 /v1/audio/transcriptions，结果直接阅读');
    } else if (c.kind === 'video') {
      body = '<h3>视频输出</h3><div class="ratio-preview">'+icon('play')+'<span>生成任务 → 直接播放</span></div>'+capabilityItem('video','封面 · 进度 · 全屏','生成状态与播放器状态分开');
    } else if (c.kind === 'database' || c.kind === 'cli') {
      body = '<h3>只读探针</h3><div class="connection-track"><span>本服务后端</span>'+icon('arrow')+'<span>目标</span></div>'+capabilityItem('shield','受控探针', c.probe || '不开放任意命令');
    } else {
      body = '<h3>对象与媒体预览</h3><div class="storage-symbol"><span>'+icon('image')+'</span><span>'+icon('audio')+'</span><span>'+icon('video')+'</span></div>'+capabilityItem('box','按文件类型直接展示','图片、音频、视频各有组件')+capabilityItem('shield','只读访问','不提供上传、删除或覆盖入口');
    }
    const note = caps.verified ? '该配置有一次与当前版本匹配的成功验证。' : '配置存在只表示已登记；连接状态要等本次测试返回。';
    return `<aside class="capability-pane">${body}<div class="capability-note">${esc(note)}</div></aside>`;
  }
  const capabilityItem = (ic,title,desc) => `<div class="capability-item">${icon(ic)}<div><strong>${esc(title)}</strong><span>${esc(desc)}</span></div></div>`;

  function renderShell() {
    if (!selectedGroup || !selectedPage) return;
    $('#primary-nav').innerHTML = catalog.groups.map(g => `<button data-group="${esc(g.id)}" class="${g.id===selectedGroup.id?'active':''}" ${g.id===selectedGroup.id?'aria-current="page"':''}>${icon(g.icon)}${esc(g.label)}</button>`).join('');
    $('#provider').innerHTML = `<div class="provider-mark">${esc(selectedGroup.mark)}</div><div><strong>${esc(selectedGroup.label)}</strong><small>${esc(selectedGroup.eyebrow)}</small></div>`;
    $('#secondary-nav').innerHTML = selectedGroup.pages.map(p=>`<button data-page="${esc(p.id)}" class="secondary-button ${p.id===selectedPage.id?'active':''}" ${p.id===selectedPage.id?'aria-current="page"':''}>${icon(p.icon)}<span><strong>${esc(p.label)}</strong><small>${esc(p.en)}</small></span></button>`).join('');
    $('#page-header').innerHTML = `<div class="page-heading"><div><div class="breadcrumb">配置中心<span class="sep">/</span>${esc(selectedGroup.label)}<span class="sep">/</span>${esc(selectedPage.label)}</div><h1>${esc(selectedPage.label)}</h1><p class="page-description">${esc(selectedPage.description)}</p></div><div class="page-identifier">${esc((selectedPage.en||'').toUpperCase())}</div></div>`;
    if (!selectedPage.configs.length) {
      $('#configs').innerHTML = '<div class="empty-result"><strong>暂无配置</strong><p>在配置数据库中登记后，这里会自动展示。页面不提供新增入口。</p></div>';
    } else {
      $('#configs').innerHTML = `<div class="config-count"><span>${selectedPage.configs.length} 项配置</span><span class="tiny-line"></span><span>数据库驱动 · 无编辑入口</span></div>` + selectedPage.configs.map(c => {
        const s = getSession(c);
        const subtitle = usesModelCombo(c) ? `${(c.models||[]).length} 个可选模型 · 可手输` : (c.model || c.id);
        const picker = usesImagePicker(c) ? pickerHTML(c, s) : '';
        const modelRow = (usesModelCombo(c) || usesImagePicker(c)) ? `<div class="field-wide model-field"><dt>模型</dt><dd>${picker}${comboHTML(c, s.model)}</dd></div>` : '';
        const props = propertiesOf(c).filter(([k]) => k !== '已登记模型' && k !== '可选模型' && k !== '模型');
        return `
        <article class="card"><header class="card-header"><span class="card-icon">${icon(typeIcon[c.kind]||'box')}</span><div class="card-title"><h2>${esc(c.title||c.label)}</h2><small>${esc(subtitle)}</small></div><span class="status-chip">${esc(statusLabel[c.status]||'未实测')}</span><button data-test="${esc(c.id)}" class="button primary">${icon('play')}${c.kind==='database'||c.kind==='cli'?'测试连接':'测试效果'}</button></header>
        <div class="card-body"><section class="config-data"><div class="data-label">${icon('lock')}配置详情 · 只读</div><dl class="property-grid">${modelRow}${props.map(([k,v])=>`<div><dt>${esc(k)}</dt><dd class="${/[a-zA-Z]{3}/.test(v)?'mono':''}">${esc(v)}</dd></div>`).join('')}</dl><div class="credential"><span>认证凭据</span><strong>${esc(credLabel[c.credentialState]||c.credentialState||'未说明')}</strong></div></section>${capabilityPane(c)}</div>
        <footer class="card-bottom"><span>测试仅使用本次输入，不修改原始配置</span><span>${esc(c.id)} · ${esc(c.revision)}</span></footer></article>`;
      }).join('');
    }
    $('#bottom-note').textContent = '配置来自 shared_config 数据库的只读投影。密钥不会下发到浏览器。测试结果是后端实际调用返回，失败时保持失败，不会用演示素材顶替。';
  }

  function route() {
    const parts = location.hash.replace(/^#\/?/,'').split('/').filter(Boolean);
    selectedGroup = catalog.groups.find(g=>g.id===parts[0]) || catalog.groups[0];
    selectedPage = selectedGroup?.pages.find(p=>p.id===parts[1]) || selectedGroup?.pages[0];
    renderShell();
  }
  function navigate(gid,pid) {
    const target=`#/${gid}/${pid||''}`;
    if (location.hash===target) route(); else location.hash=target;
  }

  function buildInput(c,s) {
    const promptLabel = c.kind==='audio'?'要朗读的文本':c.kind==='chat'?'输入你的内容':'描述想要生成的内容';
    const placeholder = {chat:'例如：用三句话解释这个配置中心的用途。',image:'例如：深海蓝控制台，细腻的青色光线。',audio:'请输入要转换成语音的文字。',video:'例如：镜头缓慢推进，青色数据光点在深蓝空间中移动。'};
    if (c.kind==='transcribe') {
      const modelHtml = usesModelCombo(c) ? `<label class="test-field-label">本次使用的模型</label>${comboHTML(c, s.model)}` : '';
      $('#input-fields').innerHTML = modelHtml+`<label class="test-field-label" for="audio-file">上传要识别的音频</label><input id="audio-file" class="file-input" type="file" accept="audio/*,.mp3,.wav,.m4a,.webm,.ogg,.mp4" /><p class="file-name" id="audio-file-name">${esc(s.filename || '尚未选择文件')}</p><p class="helper-text">仅用于本次听写测试，不会写回配置。请使用真实录音，不要用演示素材顶替结果。</p><div class="fixed-spec"><strong>当前配置</strong>${esc(s.model || c.model)}<br>接口 /v1/audio/transcriptions</div>`;
      const file=$('#audio-file');
      if(file) file.addEventListener('change',()=>readAudioFile(s,file));
    } else if (['chat','image','audio','video'].includes(c.kind)) {
      const modeHtml = c.kind==='chat' ? `<label class="test-field-label">本次测试的返回方式</label><div class="mode-pills">${modes.map(m=>`<button class="mode-pill ${s.mode===m.id?'active':''}" data-mode="${m.id}" aria-pressed="${s.mode===m.id}"><span>${esc(m.label)}</span><small>${esc(m.hint)}</small></button>`).join('')}</div>` : '';
      const modelHtml = (usesModelCombo(c) || usesImagePicker(c)) ? `<label class="test-field-label">本次使用的模型</label>${usesImagePicker(c)?pickerHTML(c,s):''}${comboHTML(c, s.model)}` : '';
      $('#input-fields').innerHTML = modeHtml+modelHtml+`<label class="test-field-label" for="prompt">${promptLabel}</label><textarea id="prompt" class="textarea" maxlength="4000" placeholder="${esc(placeholder[c.kind]||'')}">${esc(s.input)}</textarea><p class="helper-text">仅用于本次测试，不会写回配置。<br>点击开始后才会调用已登记服务，可能产生费用。</p><div class="fixed-spec"><strong>当前配置</strong>${esc(s.model || c.model)}<br>版本 ${esc(c.revision)}</div>`;
    } else if (c.kind==='database' || c.kind==='cli') {
      $('#input-fields').innerHTML = `<label class="test-field-label">测试范围</label><div class="fixed-spec"><strong>预设的只读探针</strong>${esc(c.probe||'受控检查')}<br>不执行任意输入的命令</div><p class="helper-text">由服务端执行白名单探针，返回真实连接状态与耗时。</p><div class="fixed-spec"><strong>目标配置</strong>${esc(c.title||c.label)}<br>${esc(c.model)}</div>`;
    } else {
      $('#input-fields').innerHTML = `<label class="test-field-label" for="prompt">对象名称或前缀</label><textarea id="prompt" class="textarea" maxlength="4000" placeholder="留空则使用配置中的默认前缀">${esc(s.input)}</textarea><p class="helper-text">只读列举或预览。不会写入或删除对象。</p><div class="fixed-spec"><strong>媒体展示方式</strong>图片：缩略图与大图<br>视频：内嵌播放器<br>语音：内嵌音频播放器</div>`;
    }
    $('#test-disclaimer').textContent = '测试输入不会写回配置库。结果区只展示本次运行的真实输出。';
    const runLabel = {database:'运行探针',cli:'检查可执行文件',storage:'列举对象',media:'预览对象',transcribe:'开始听写'}[c.kind] || '开始测试';
    $('#run-test').innerHTML = icon('play')+runLabel;
    const prompt=$('#prompt');
    if(prompt) prompt.addEventListener('input',()=>{s.input=prompt.value;});
  }

  function readAudioFile(s, input) {
    const file = input.files && input.files[0];
    const label = $('#audio-file-name');
    if (!file) {
      s.audioBase64 = '';
      s.filename = '';
      if (label) label.textContent = '尚未选择文件';
      return;
    }
    if (file.size > 8 * 1024 * 1024) {
      toast('音频请小于 8MB。');
      input.value = '';
      return;
    }
    const reader = new FileReader();
    reader.onload = () => {
      const result = String(reader.result || '');
      const comma = result.indexOf(',');
      s.audioBase64 = comma >= 0 ? result.slice(comma + 1) : result;
      s.filename = file.name;
      if (label) label.textContent = file.name;
    };
    reader.readAsDataURL(file);
  }

  function openTest(id,trigger) {
    activeConfig = selectedPage.configs.find(c=>c.id===id);
    if (!activeConfig) return;
    activeGroup=selectedGroup; activePage=selectedPage;
    returnFocus=trigger;
    const c=activeConfig,s=getSession(c);
    $('#test-title').textContent = `${typeName[c.kind]||'效果'}测试`;
    $('#test-icon').innerHTML = icon(typeIcon[c.kind]||'play');
    $('#test-context').innerHTML = `<span>${esc(activeGroup.label)}</span><span class="divider">/</span><span>${esc(c.title||c.label)}</span><span class="divider">·</span><span>${esc(s.model || c.model)}</span><span class="context-tail">绑定版本 ${esc(c.revision)} · 不改变数据库配置</span>`;
    buildInput(c,s);
    renderResult();
    if(!dialog.open) dialog.showModal();
    setTimeout(()=>($('#prompt')||$('#run-test')).focus(),80);
  }

  function pauseMedia(root=document) {root.querySelectorAll('audio,video').forEach(el=>{try{el.pause();}catch(_){}});}
  function closeTest() { pauseMedia(dialog); dialog.close(); if(returnFocus?.isConnected) returnFocus.focus(); }

  function syncRunning() {
    if (!activeConfig) return;
    const s=getSession(activeConfig), running=s.status==='running'||s.status==='submitting';
    $('#run-test').hidden=running; $('#stop-test').hidden=!running;
    $('#input-fields').querySelectorAll('textarea,button,input').forEach(el=>el.disabled=running);
    const labels={idle:'等待输入',submitting:'正在提交',running:'调用中',done:'已完成',error:'失败',cancelled:'已停止等待'};
    $('#run-status').textContent = (s.run && s.run.stageLabel) || labels[s.status] || '等待输入';
    $('#live-badge').hidden = !(s.status==='done' && s.outputs.length);
  }

  function emptyHtml(c) {
    const hints={
      chat:'输入内容后，回答会出现在这里。\n流式内容按事件追加；一次性在完成后显示。',
      image:'返回的图片直接展示。\n点击图片即可放大。',
      audio:'合成完成后，在这里点击试听。',
      transcribe:'识别完成后，转写文字会出现在这里。',
      video:'任务完成后，在这里直接播放。',
      database:'运行只读探针后，显示状态、耗时与受控结果。',
      cli:'检查命令文件是否存在，不会代跑完整任务。',
      storage:'读取对象后，按媒体类型直接预览。',
      media:'图片、音频与视频都在这里预览。'
    };
    return `<div class="empty-result"><span class="empty-mark">${icon(typeIcon[c.kind]||'box')}</span><strong>让结果直接可见</strong><p>${esc(hints[c.kind]||'').replace(/\n/g,'<br>')}</p></div>`;
  }

  function treeNode(value,key='result') {
    if (value!==null && typeof value==='object') {
      return `<details open><summary>${esc(key)} <span style="color:var(--quiet)">${Array.isArray(value)?`[${value.length}]`:'{ }'}</span></summary><div style="margin-left:12px">${Object.entries(value).map(([k,v])=>treeNode(v,k)).join('')}</div></details>`;
    }
    return `<div class="leaf"><b>${esc(key)}:</b>${esc(JSON.stringify(value))}</div>`;
  }

  function assetSrc(asset) {
    if (!asset) return '';
    if (asset.access?.type==='same_origin') return asset.access.path;
    if (asset.access?.type==='signed_url') return asset.access.url;
    return `/api/media/${asset.assetId}`;
  }

  function mediaErrorHook(root) {
    root.querySelectorAll('audio,video,img').forEach(el=>{
      el.addEventListener('error',()=>{
        if(el.dataset.errorShown) return; el.dataset.errorShown='1';
        const p=document.createElement('p'); p.className='result-note';
        p.textContent='该媒体无法播放或显示。这是真实返回文件的加载失败，不是演示素材。';
        el.insertAdjacentElement('afterend',p);
      });
    });
  }

  function renderOutputs(c,s) {
    const blocks = s.outputs || [];
    if (!blocks.length && (s.status==='running'||s.status==='submitting')) {
      return `<div class="loader-wrap"><span class="spinner" aria-hidden="true"></span>${esc(s.run?.stageLabel||'正在调用已登记服务')}<p>不会插入虚构百分比，也不会用本地样例顶替。</p></div>`;
    }
    return blocks.map(block => renderBlock(c,s,block)).join('');
  }

  function renderBlock(c,s,block) {
    if (block.kind==='text') {
      const streaming = s.status==='running' && !block.complete;
      return `<div class="message assistant"><span class="message-label">模型返回</span><span class="${streaming?'stream-cursor':''}">${esc(block.text||'')}</span></div>`;
    }
    if (block.kind==='json') {
      let parsed = block.parsed;
      if (parsed==null && block.validJson && block.rawText) {
        try { parsed = JSON.parse(block.rawText); } catch(_) {}
      }
      const tree = parsed!=null ? treeNode(parsed) : `<div class="leaf">无法解析为 JSON</div>`;
      const raw = esc(block.rawText || JSON.stringify(parsed,null,2) || '');
      const valid = block.validJson ? '合法 JSON' : 'JSON 解析失败，仍展示原文';
      return `<h3 class="result-title">结构化返回 · ${esc(valid)}</h3><div class="json-switch"><button data-json="tree" class="${s.jsonView==='tree'?'active':''}">结构视图</button><button data-json="raw" class="${s.jsonView==='raw'?'active':''}">格式化源码</button></div>${s.jsonView==='tree'?`<div class="json-tree">${tree}</div>`:`<pre class="json-box">${raw}</pre>`}`;
    }
    if (block.kind==='image') {
      const assets = block.assets || [];
      if (!assets.length) return `<p class="result-note">没有可展示的图片。</p>`;
      return `<h3 class="result-title">图片结果</h3>`+assets.map(a=>`<button class="result-image" data-enlarge="${esc(assetSrc(a))}" aria-label="放大图片结果"><img src="${esc(assetSrc(a))}" alt="本次测试返回的图片" /></button><div class="asset-meta"><strong>${esc(a.filename||a.mimeType||'image')}</strong><span>${a.width&&a.height?`${a.width} × ${a.height} · `:''}${esc(a.mimeType||'')}${a.sizeBytes?` · ${a.sizeBytes} bytes`:''}</span></div>`).join('');
    }
    if (block.kind==='audio' && block.asset) {
      const a=block.asset;
      return `<div class="audio-hero"><div class="audio-title">${icon('audio')}语音已就绪</div><div class="audio-sub">${esc(a.mimeType||'audio')} · 本次合成结果</div><audio controls preload="metadata" src="${esc(assetSrc(a))}" aria-label="播放本次合成的语音"></audio></div>${block.transcript?`<div class="transcript"><span>本次朗读文本</span>${esc(block.transcript)}</div>`:''}`;
    }
    if (block.kind==='video' && block.asset) {
      const a=block.asset;
      return `<h3 class="result-title">视频结果</h3><video class="sample-video" controls playsinline preload="metadata" src="${esc(assetSrc(a))}" aria-label="播放本次生成的视频"></video><div class="asset-meta"><strong>${esc(a.filename||'video')}</strong><span>${esc(a.mimeType||'video')}${a.sizeBytes?` · ${a.sizeBytes} bytes`:''}</span></div>`;
    }
    if (block.kind==='table') {
      const head = (block.columns||[]).map(col=>`<th scope="col">${esc(col)}</th>`).join('');
      const body = (block.rows||[]).map(row=>`<tr>${row.map(cell=>`<td>${esc(cell)}</td>`).join('')}</tr>`).join('');
      return `<h3 class="result-title">探针结果</h3><table class="result-table"><thead><tr>${head}</tr></thead><tbody>${body}</tbody></table>`;
    }
    if (block.kind==='diagnostic') {
      const rows = (block.checks||[]).map(ch=>`<tr><td>${esc(ch.label)}</td><td>${esc(ch.status)} · ${esc(ch.detail||'')}</td></tr>`).join('');
      return `<table class="result-table"><thead><tr><th>检查项</th><th>结果</th></tr></thead><tbody>${rows}</tbody></table>`;
    }
    return '';
  }

  function renderResult() {
    if(!activeConfig) return;
    const c=activeConfig,s=getSession(c),target=$('#result');
    syncRunning();
    if(s.status==='idle'){target.innerHTML=emptyHtml(c);return;}
    if(s.status==='error' && !s.outputs.length){
      const msg = s.error?.safeMessage || s.error?.detail || '本次测试失败';
      const rid = s.error?.requestId ? `<p class="result-note">请求编号 ${esc(s.error.requestId)}</p>` : '';
      target.innerHTML=`<div class="empty-result"><strong>本次测试失败</strong><p>${esc(msg)}</p>${rid}</div>`;
      return;
    }
    if(s.status==='cancelled' && !s.outputs.length){
      target.innerHTML='<div class="empty-result"><strong>已停止等待</strong><p>上游可能仍在运行并计费。页面不会把这个状态显示成“已取消成功”。</p></div>';
      return;
    }
    let html = '';
    if (c.kind==='chat' && s.messages.length && s.mode!=='json') {
      html += s.messages.map(m=>`<div class="message ${m.role}"><span class="message-label">${m.role==='user'?'你的输入':'模型返回'}</span><span>${esc(m.text)}</span></div>`).join('');
    }
    html += renderOutputs(c,s);
    if (s.status==='error' && s.error) {
      html += `<p class="result-note">失败：${esc(s.error.safeMessage||'未知错误')} · ${esc(s.error.requestId||'')}</p>`;
    }
    if (s.status==='cancelled') {
      html += `<p class="result-note">已停止等待。上游可能仍在运行并计费。</p>`;
    }
    target.innerHTML = html || emptyHtml(c);
    mediaErrorHook(target);
    if (s.status==='running') target.scrollTop = target.scrollHeight;
  }

  function applyEvent(c,s,event) {
    if (!event) return;
    if (event.type==='status') {
      if (s.run) { s.run.state = event.state; s.run.stageLabel = event.stageLabel; }
      s.status = event.state==='succeeded'?'running': (event.state||s.status);
      if (event.state==='running') s.status='running';
    }
    if (event.type==='text_delta') {
      s.status='running';
      let block = s.outputs.find(b=>b.id===event.blockId);
      if (!block) { block={id:event.blockId,kind:'text',text:'',complete:false}; s.outputs.push(block); }
      block.text = (block.text||'') + (event.delta||'');
    }
    if (event.type==='output' && event.block) {
      const idx = s.outputs.findIndex(b=>b.id===event.block.id);
      if (idx>=0) s.outputs[idx]=event.block; else s.outputs.push(event.block);
    }
    if (event.type==='error') {
      s.error = {code:event.code, safeMessage:event.safeMessage, requestId:event.requestId};
    }
    if (event.type==='done' && event.run) {
      s.run = event.run;
      s.outputs = event.run.outputs || s.outputs;
      if (event.run.state==='succeeded') s.status='done';
      else if (event.run.error?.code==='STOPPED_WAITING') s.status='cancelled';
      else s.status='error', s.error = event.run.error || s.error;
    }
  }

  async function consumeEvents(s, runId) {
    const res = await fetch(`/api/test-runs/${encodeURIComponent(runId)}/events`, {headers:{'Accept':'text/event-stream'}});
    if (!res.ok || !res.body) throw new Error(await readError(res));
    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buf = '';
    s.abort = () => reader.cancel();
    while (true) {
      const {value, done} = await reader.read();
      if (done) break;
      buf += decoder.decode(value, {stream:true});
      const chunks = buf.split('\n\n');
      buf = chunks.pop() || '';
      for (const chunk of chunks) {
        const dataLine = chunk.split('\n').find(line => line.startsWith('data:'));
        if (!dataLine) continue;
        let event;
        try { event = JSON.parse(dataLine.slice(5).trim()); } catch { continue; }
        applyEvent(activeConfig && activeConfig.id===s.configId ? activeConfig : {id:s.configId}, s, event);
        if (activeConfig && getSession(activeConfig)===s) renderResult();
      }
    }
  }

  async function readError(res) {
    try {
      const body = await res.json();
      return body.detail || body.title || `HTTP ${res.status}`;
    } catch {
      return `HTTP ${res.status}`;
    }
  }

  function idempotencyKey() {
    if (crypto.randomUUID) return crypto.randomUUID();
    return `k-${Date.now()}-${Math.random().toString(16).slice(2)}`;
  }

  async function runTest() {
    if(!activeConfig) return;
    const c=activeConfig,s=getSession(c),input=$('#prompt');
    if(input) s.input=input.value;
    if(['chat','image','audio','video'].includes(c.kind)&&!s.input.trim()){toast('请先输入本次测试的内容。');input?.focus();return;}
    if(c.kind==='transcribe'&&!s.audioBase64){toast('请先上传要识别的音频。');return;}
    if(s.status==='running'||s.status==='submitting') return;
    pauseMedia(dialog);
    s.status='submitting'; s.error=null; s.outputs=[]; s.run=null; s.configId=c.id;
    if (c.kind==='chat' && s.mode!=='json') s.messages = [{role:'user',text:s.input}];
    else s.messages = [];
    renderResult();
    const preset = modes.find(m=>m.id===s.mode) || modes[0];
    const body = {
      configId: c.id,
      expectedRevision: c.revision,
      idempotencyKey: idempotencyKey(),
      input: buildInputPayload(c,s),
      outputPreset: c.kind==='chat' ? {delivery: preset.delivery, format: preset.format} : undefined
    };
    try {
      const res = await fetch('/api/test-runs', {method:'POST', headers:{'Content-Type':'application/json'}, body: JSON.stringify(body)});
      if (!res.ok) throw new Error(await readError(res));
      const payload = await res.json();
      s.run = payload.run;
      s.runId = payload.run.id;
      s.status='running';
      renderResult();
      await consumeEvents(s, s.runId);
      if (s.status==='running') {
        const latest = await fetch(`/api/test-runs/${encodeURIComponent(s.runId)}`);
        if (latest.ok) {
          const run = await latest.json();
          applyEvent(c,s,{type:'done', run});
        }
      }
    } catch (err) {
      if (s.status!=='cancelled') {
        s.status='error';
        s.error = {safeMessage: String(err.message || err), requestId: s.runId || ''};
      }
    } finally {
      s.abort = null;
      if (activeConfig?.id===c.id) renderResult();
    }
  }

  function buildInputPayload(c,s) {
    const model = (s.model || c.model || '').trim();
    if (c.kind==='chat') return {kind:'chat', model, messages:[{role:'user', content:s.input}]};
    if (c.kind==='image' || c.kind==='video') return {kind:c.kind, model, prompt:s.input};
    if (c.kind==='audio') return {kind:'audio', model, text:s.input};
    if (c.kind==='transcribe') return {kind:'transcribe', model, audioBase64:s.audioBase64, filename:s.filename};
    if (c.kind==='database') return {kind:'database', probeId: (c.probe||'').toUpperCase()==='PING'?'ping':'select_one'};
    if (c.kind==='cli') return {kind:'database', probeId:'connect'};
    return {kind: c.kind==='media'?'media':'storage', prefix:s.input, objectKey:s.input};
  }

  async function stopTest() {
    if(!activeConfig) return;
    const s=getSession(activeConfig);
    if (s.abort) { try { s.abort(); } catch(_){} }
    if (s.runId) {
      try { await fetch(`/api/test-runs/${encodeURIComponent(s.runId)}/cancel`, {method:'POST'}); } catch(_){}
    }
    s.status='cancelled';
    renderResult();
  }

  document.addEventListener('click',event=>{
    const option=event.target.closest('.combo-option');
    if(option){
      event.preventDefault();
      applyModel(option.dataset.config, option.dataset.model);
      closeCombos();
      return;
    }
    const toggle=event.target.closest('[data-combo-toggle]');
    if(toggle){
      event.preventDefault();
      const menu=toggle.closest('.combo').querySelector('.combo-menu');
      const willOpen=menu.hidden;
      closeCombos();
      menu.hidden=!willOpen;
      return;
    }
    if(!event.target.closest('.combo')) closeCombos();
    const el=event.target.closest('button'); if(!el) return;
    if(el.dataset.group){const g=catalog.groups.find(g=>g.id===el.dataset.group);navigate(g.id,g.pages[0]?.id||'');}
    if(el.dataset.page) navigate(selectedGroup.id, el.dataset.page);
    if(el.dataset.test) openTest(el.dataset.test, el);
    if(el.dataset.mode&&activeConfig){const s=getSession(activeConfig); if(s.status==='running'||s.status==='submitting') return; s.mode=el.dataset.mode; buildInput(activeConfig,s);}
    if(el.dataset.json&&activeConfig){getSession(activeConfig).jsonView=el.dataset.json; renderResult();}
    if(el.dataset.enlarge){$('#large-image').src=el.dataset.enlarge; $('#image-caption').textContent='图片预览 · 本次测试返回'; bigImage.showModal();}
  });
  $('#run-test').addEventListener('click',runTest);
  $('#stop-test').addEventListener('click',stopTest);
  $('#close-test').innerHTML=icon('close'); $('#close-test').addEventListener('click',closeTest);
  $('#expand-test').innerHTML=icon('expand'); $('#expand-test').addEventListener('click',()=>{
    const full=dialog.classList.toggle('full');
    $('#expand-test').innerHTML=icon(full?'collapse':'expand');
    $('#expand-test').setAttribute('aria-label',full?'还原侧栏尺寸':'展开为全屏');
  });
  $('#close-image').innerHTML=icon('close'); $('#close-image').addEventListener('click',()=>bigImage.close());
  dialog.addEventListener('close',()=>pauseMedia(dialog));
  document.addEventListener('keydown',event=>{
    if(dialog.open&&!bigImage.open&&(event.ctrlKey||event.metaKey)&&event.key==='Enter'){event.preventDefault(); runTest();}
  });
  document.addEventListener('change', event => {
    const select = event.target.closest('[data-param]');
    if (!select) return;
    const grid = select.closest('.param-grid');
    if (!grid) return;
    const cfg = findConfigById(grid.dataset.config);
    if (!cfg) return;
    const s = getSession(cfg);
    s.family = (grid.querySelector('[data-param="family"]') || {}).value || 'flash-3.1';
    s.resolution = (grid.querySelector('[data-param="resolution"]') || {}).value || '';
    s.aspect = (grid.querySelector('[data-param="aspect"]') || {}).value || '';
    applyModel(cfg.id, composeAntigravityImage(s.family, s.resolution, s.aspect));
  });
  document.addEventListener('input', event => {
    const input = event.target.closest('.combo-input');
    if (!input) return;
    const cfg = findConfigById(input.dataset.config);
    if (!cfg) return;
    getSession(cfg).model = input.value.trim();
    filterCombo(input.closest('.combo'), input.value);
  });
  window.addEventListener('hashchange',()=>{ if(catalog) route(); });

  async function init(){
    const res = await fetch('/api/catalog', {cache:'no-store'});
    if (!res.ok) throw new Error(await readError(res));
    catalog = await res.json();
    $('#source-label').innerHTML = `<span class="dot"></span>${esc(catalog.source||'postgres')} · 只读`;
    if(!catalog.groups?.length){
      $('#main').insertAdjacentHTML('beforeend','<div class="empty-result"><strong>数据库中没有可展示的配置分组</strong><p>请通过已有配置管理流程登记后再刷新。本页不提供新增入口。</p></div>');
      return;
    }
    route();
    document.documentElement.dataset.ready='true';
  }
  init().catch(error=>{
    $('#source-label').innerHTML = `<span class="dot amber"></span>读取失败`;
    $('#configs').innerHTML = `<div class="empty-result"><strong>无法读取配置数据库</strong><p>${esc(error.message)}</p></div>`;
  });
})();
