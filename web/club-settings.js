const key='cookiekill-club-reduced-flashing';
let preference=null;
let media=null;
try {const saved=globalThis.localStorage?.getItem(key);if(saved!==null&&saved!==undefined)preference=saved==='true';} catch {}
export function clubReducedFlashing(){if(!media&&globalThis.matchMedia)media=globalThis.matchMedia('(prefers-reduced-motion: reduce)');return !!media?.matches||preference===true;}
export function setClubReducedFlashing(value){preference=!!value;try{globalThis.localStorage?.setItem(key,String(preference));}catch{}return clubReducedFlashing();}
