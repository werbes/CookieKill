import {achievementPaintingDataURL} from './achievement-painting-art.js';

export function createAchievementPanels({state,button,escapeHTML}){
  const esc=value=>escapeHTML(String(value??''));
  const catalog=()=>state.snapshot?.achievementCatalog||[],hatsCatalog=()=>state.snapshot?.hatCatalog||[],me=()=>state.snapshot?.me||{};
  function hats(draft){
    const p=me();
    return `<div class="section-label">ACHIEVEMENT HATS <span>Earn an achievement, then wear its hat</span></div><div class="hat-options">${button(!draft.hat?'✓ No hat':'No hat','avatar-hat',{hat:''})}${hatsCatalog().map(h=>{
      const unlocked=!!p.hats?.[h.id],achievement=catalog().find(a=>a.id===h.achievement);
      return `<div class="hat-option${unlocked?'':' locked'}">${button((draft.hat===h.id?'✓ ':'')+h.name,'avatar-hat',{hat:h.id},!unlocked)}<span>${esc(unlocked?'Unlocked':achievement?.description||'Earn the matching achievement.')}</span></div>`;
    }).join('')}</div>`;
  }
  function preview(id){return hatsCatalog().find(h=>h.id===id)?.icon||'';}
  function gallery({paintings=false}={}){
    const p=me(),earned=p.achievements||{},valid=new Set(catalog().map(a=>a.id)),hung=new Set((p.hungAchievements||[]).filter(id=>valid.has(id)&&earned[id]));
    const count=hung.size,factor=(1+count/10).toFixed(1),plot=state.snapshot?.nodes?.find(n=>n.id===p.bakeryPlot&&n.kind==='bakery_plot'&&n.owner===p.id);
    const near=state.connected!==false&&p.bakery&&!!plot&&!p.island&&!p.inCave&&Math.abs(p.y||0)<=2&&Math.hypot(p.x-plot.x,p.z-plot.z)<=10;
    const total=catalog().filter(a=>earned[a.id]).length;
    if(paintings)return `<div class="section-label">ACHIEVEMENT PAINTINGS <span>${total} / ${catalog().length} earned &middot; ${count} hanging</span></div><p class="panel-intro">Turn your achievements into paintings for your bakery's gallery wall. Each earned painting has its own wall space. Hang it once, or take it down whenever you like. Paintings stay with your bakery when you reopen on another plot.</p><div class="notice good"><strong>Achievement price factor: ${factor}&times;</strong><p>Each distinct earned painting adds 10% of the original customer order price. 1 + 0.10 &times; ${count} = ${factor}&times;. Four paintings give 1.4&times;. Other bakery bonuses are added separately; the combined achievement bonus rounds down once to whole coins.</p></div>${near?'<p class="field-hint">Choose Hang painting to place it on the bakery gallery wall. You can hang or remove paintings in either explore mode or build mode.</p>':'<p class="field-hint">Visit your own bakery sign or interior to hang or take down paintings.</p>'}<div class="recipe-grid achievement-painting-catalog">${catalog().map(a=>{
      const unlocked=!!earned[a.id],displayed=hung.has(a.id),status=displayed?'Hanging in your bakery':unlocked?'Earned &middot; Ready to hang':'Locked &middot; Earn this achievement first';
      return `<article class="recipe-card achievement-painting-card${unlocked?'':' locked'}"><div class="achievement-painting-preview" aria-hidden="true"><img src="${achievementPaintingDataURL(a.id)}" alt=""></div><h3>${esc(a.name)}</h3><p>${esc(a.description)}</p><p class="field-hint">${status}</p><div class="item-actions">${button(displayed?'Take down painting':'Hang painting',displayed?'unhang_achievement':'hang_achievement',{item:a.id},!near||!unlocked,!displayed&&unlocked)}</div></article>`;
    }).join('')}</div>`;
    return `<div class="section-label">YOUR ACHIEVEMENTS <span>${total} / ${catalog().length} earned · ${count} hanging</span></div><p class="field-hint">Achievements and their hats stay earned permanently. Hang each achievement once on your bakery's gallery wall. Every distinct earned frame adds 10% of the original customer price.</p><div class="notice good"><strong>Achievement price factor: ${factor}×</strong><p>1 + 0.10 × ${count} = ${factor}× the original price. Four frames give 1.4×. Other bakery bonuses are added separately; the combined achievement bonus rounds down once to whole coins.</p></div>${near?'':'<p class="field-hint">Visit your own bakery sign or interior to hang or remove frames.</p>'}<div class="shop-list">${catalog().map(a=>{
      const unlocked=!!earned[a.id],displayed=hung.has(a.id),hat=hatsCatalog().find(h=>h.id===a.hat);
      return `<div class="notice${unlocked?' good':''}"><strong>${esc(a.icon)} ${esc(a.name)}</strong><p>${esc(a.description)}</p><p class="field-hint">${unlocked?'Earned':'Locked'} · ${esc(hat?.name||'Achievement hat')}${displayed?' · Hanging in your bakery':''}</p><div class="item-actions">${button(displayed?'Take down frame':'Hang in bakery',displayed?'unhang_achievement':'hang_achievement',{item:a.id},!near||!unlocked,!displayed&&unlocked)}</div></div>`;
    }).join('')}</div>`;
  }
  if(typeof document!=='undefined'&&!document.getElementById('achievement-paintings-style')){const style=document.createElement('style');style.id='achievement-paintings-style';style.textContent='.achievement-painting-card{display:flex;flex-direction:column;gap:10px}.achievement-painting-card h3,.achievement-painting-card p{margin:0}.achievement-painting-card .item-actions{margin-top:auto}.achievement-painting-card.locked .achievement-painting-preview{filter:grayscale(1);opacity:.55}.achievement-painting-preview{display:grid;place-items:center;min-height:105px;margin-bottom:4px;overflow:hidden;border:8px solid #9d7951;outline:1px solid #755938;border-radius:3px;background:linear-gradient(155deg,#eee0b7 0%,#d7e2c7 60%,#bacfa9 60%,#adc497 100%);box-shadow:inset 0 0 0 3px #f7ebc5}.achievement-painting-preview img{display:block;width:100%;aspect-ratio:16/9;object-fit:cover}.bakery-build-categories{margin:0 0 22px;gap:9px;flex-wrap:wrap}';document.head.appendChild(style);}
  return {hats,preview,gallery};
}
