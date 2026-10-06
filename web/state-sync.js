// Reconstruct full render state from one connection's snapshot/delta stream.
const entityKeys = {players:'id',nodes:'id',animals:'id',projectiles:'id',homes:'owner'};
const has = (value, key) => Object.prototype.hasOwnProperty.call(value, key);
const object = (value) => value !== null && typeof value === 'object' && !Array.isArray(value);
const sequence = (value) => Number.isSafeInteger(value) && value > 0;

function array(value, name) {
  if (value == null) return [];
  if (!Array.isArray(value)) throw new Error(`Invalid ${name} array`);
  return value;
}

function entities(value, key, name) {
  return array(value, name).map((item) => {
    if (!object(item) || typeof item[key] !== 'string' || !item[key]) throw new Error(`Invalid ${name} identity`);
    return {...item};
  });
}

function updateEntities(previous, changes, key, name) {
  if (!object(changes)) throw new Error(`Invalid ${name} changes`);
  const next = new Map(previous.map((item) => [item[key], item]));
  for (const id of array(changes.remove, `${name}.remove`)) {
    if (typeof id !== 'string') throw new Error(`Invalid ${name} removal`);
    next.delete(id);
  }
  for (const patch of entities(changes.upsert, key, `${name}.upsert`)) {
    // Patches replace nested maps/arrays, including empty ones and null values.
    next.set(patch[key], {...next.get(patch[key]), ...patch});
  }
  return [...next.values()];
}

export function createSnapshotDecoder() {
  let current = null;
  return {
    apply(frame) {
      if (!object(frame)) throw new Error('Invalid game update');
      if (frame.type !== 'snapshot' && frame.type !== 'delta') return null;
      let next;
      if (frame.type === 'snapshot') {
        if (!object(frame.me)) throw new Error('Snapshot is missing your player');
        if (has(frame, 'seq') && (!sequence(frame.seq) || (current?.seq && frame.seq <= current.seq))) {
          throw new Error('Stale or invalid snapshot sequence');
        }
        next = {...frame, me:{...frame.me}};
        for (const [name, key] of Object.entries(entityKeys)) next[name] = entities(frame[name], key, name);
        for (const name of ['recipes','events','friends','buildCatalog']) next[name] = array(frame[name], name);
      } else {
        if (!current || !sequence(current.seq) || !sequence(frame.seq) || frame.base !== current.seq || frame.seq !== current.seq + 1) {
          throw new Error('Game update sequence mismatch');
        }
        const {base, ...fields} = frame;
        next = {...current, ...fields, type:'snapshot'};
        if (has(frame, 'me')) {
          if (!object(frame.me)) throw new Error('Invalid player changes');
          next.me = {...current.me, ...frame.me};
        }
        for (const [name, key] of Object.entries(entityKeys)) {
          if (has(frame, name)) next[name] = updateEntities(current[name], frame[name], key, name);
        }
        for (const name of ['recipes', 'events', 'friends', 'buildCatalog']) {
          if (has(frame, name)) next[name] = array(frame[name], name);
        }
      }
      // Commit only after the entire frame has passed validation.
      current = next;
      return next;
    }
  };
}
