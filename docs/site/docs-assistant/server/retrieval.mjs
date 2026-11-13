const stop = new Set('a an and are as at be by can could do does for from has have how i in is it me my of on or our please the their this to use using was we what when where which will with would you your'.split(' '));
const forms = new Map([
  ['deployment', 'deploy'], ['deployments', 'deploy'], ['deployed', 'deploy'], ['deploying', 'deploy'],
  ['balances', 'balance'], ['tokens', 'token'], ['contracts', 'contract'], ['wallets', 'wallet'],
  ['files', 'file'], ['queries', 'query'], ['querying', 'query'], ['models', 'model'],
  ['images', 'image'], ['teammates', 'teammate'], ['proofs', 'proof'], ['transactions', 'transaction'],
  ['connections', 'connect'], ['connecting', 'connect'], ['connected', 'connect'],
  ['scheduling', 'schedule'], ['scheduled', 'schedule'], ['schedules', 'schedule'],
  ['guarantors', 'guarantor'], ['bonds', 'bond'], ['checkpoints', 'checkpoint'],
  ['budgets', 'budget'], ['approvals', 'approval'], ['receipts', 'receipt'],
  ['accounts', 'account'], ['activities', 'activity'], ['businesses', 'business'],
  ['markets', 'market'], ['programs', 'program'], ['subscriptions', 'subscription'],
  ['webhooks', 'webhook'], ['mandates', 'mandate'], ['attestations', 'attestation'],
  ['orders', 'order'], ['delegations', 'delegation'], ['replicas', 'replica'],
  ['returns', 'return'], ['creates', 'create'], ['creating', 'create'],
]);
export function terms(text) {
  const normalized = text.replace(/\b(erc|cw)[ -]+(\d+)\b/gi, '$1$2').replace(/([a-z])([A-Z])/g, '$1 $2').toLowerCase();
  return (normalized.match(/[\p{L}\p{N}_]+/gu) || []).filter(word => word.length > 1 && !stop.has(word)).map(word => forms.get(word) || word);
}
export function createIndex(document) {
  const chunks = [];
  for (const page of document.pages) {
    const sections = [];
    let section = '', fence = null;
    for (const line of page.content.replace(/^---\n[\s\S]*?\n---\n/, '').split('\n')) {
      const marker = /^\s*(`{3,}|~{3,})/.exec(line)?.[1];
      if (!fence && /^#{1,3} /.test(line) && section.trim()) { sections.push(section); section = ''; }
      section += line + '\n';
      if (marker && !fence) fence = marker;
      else if (marker && fence && marker[0] === fence[0] && marker.length >= fence.length) fence = null;
    }
    if (section.trim()) sections.push(section);
    let headingCursor = 0;
    for (const section of sections) {
      const heading = /^#{1,3} .*/.exec(section)?.[0] || page.title;
      const headingText = heading.replace(/^#{1,3} /, '');
      const sourceHeading = /^#{2,3} /.test(heading) ? page.headings?.slice(headingCursor).find(item => item.title === headingText) : undefined;
      if (sourceHeading) headingCursor = page.headings.indexOf(sourceHeading) + 1;
      const citationPath = sourceHeading?.id ? `${page.path}#${sourceHeading.id}` : page.path;
      let content = '', fence = null;
      for (const line of section.split('\n')) {
        const marker = /^\s*(`{3,}|~{3,})/.exec(line)?.[1];
        content += line + '\n';
        if (marker && !fence) fence = marker;
        else if (marker && fence && marker[0] === fence[0] && marker.length >= fence.length) fence = null;
        if (!fence && !line.trim() && content.length >= 3500) {
          chunks.push({ ...page, citationPath, content }); content = heading + '\n\n';
        }
      }
      if (content.trim() && content.trim() !== heading) chunks.push({ ...page, citationPath, content });
    }
  }
  const frequencies = new Map();
  for (const chunk of chunks) {
    const words = terms(chunk.content);
    chunk.length = words.length;
    chunk.tf = new Map();
    for (const word of words) chunk.tf.set(word, (chunk.tf.get(word) || 0) + 1);
    chunk.titleTerms = new Set(terms(chunk.title));
    chunk.headingTerms = new Set(terms(/^#{1,3} (.+)$/m.exec(chunk.content)?.[1] || ''));
    for (const word of chunk.tf.keys()) frequencies.set(word, (frequencies.get(word) || 0) + 1);
  }
  return { ...document, chunks, frequencies, average: chunks.reduce((sum, chunk) => sum + chunk.length, 0) / (chunks.length || 1) };
}
export function retrieve(index, messages, pagePath) {
  const current = messages.at(-1).content;
  const query = [...new Set(terms(current))];
  const history = [...new Set(messages.filter(m => m.role === 'user').slice(-3, -1).flatMap(m => terms(m.content)))];
  const weights = new Map(history.map(word => [word, 0.25]));
  for (const word of query) weights.set(word, 1);
  const normalize = path => path.replace(/\/$/, '') || '/';
  const ranked = index.chunks.map(chunk => {
    let score = 0;
    for (const [word, weight] of weights) {
      const tf = chunk.tf.get(word) || 0;
      if (tf) {
        const df = index.frequencies.get(word) || 0;
        const idf = Math.log(1 + (index.chunks.length - df + 0.5) / (df + 0.5));
        score += weight * idf * tf * 2.4 / (tf + 1.4 * (0.25 + 0.75 * chunk.length / index.average));
      }
      if (chunk.titleTerms.has(word)) score += 2 * weight;
      if (chunk.headingTerms.has(word)) score += 1.5 * weight;
    }
    if (normalize(chunk.path) === normalize(pagePath)) score += 1.5;
    return { chunk, score };
  }).sort((a, b) => b.score - a.score);
  const selected = [], counts = new Map();
  let characters = 0;
  for (const result of ranked) {
    if (selected.length >= 7) break;
    if (result.score <= 0 || (counts.get(result.chunk.path) || 0) >= 3) continue;
    if (selected.length && characters + result.chunk.content.length > 30000) continue;
    characters += result.chunk.content.length;
    selected.push(result.chunk); counts.set(result.chunk.path, (counts.get(result.chunk.path) || 0) + 1);
  }
  return selected;
}
