'use client'

import { useRef, useState } from 'react'
import { CopyControl } from '@centra/docs-assistant/react'
import Link from 'next/link'

type Language = 'cURL' | 'JavaScript' | 'Python'
export function DeveloperQuickstart() {
  const [language, setLanguage] = useState<Language>('cURL')
  const [endpoint, setEndpoint] = useState('http://localhost:8545')
  const [copyFeedback, setCopyFeedback] = useState('')
  const [validation, setValidation] = useState('')
  const [interacted, setInteracted] = useState(false)
  const input = useRef<HTMLInputElement>(null)
  const payload = '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}'
  const quote = (value: string) => "'" + value.replaceAll("'", "'\\''") + "'"
  const snippets: Record<Language, string> = {
    cURL: `curl ${quote(endpoint)} \\\n  -H 'Content-Type: application/json' \\\n  --data '${payload}'`,
    JavaScript: `const response = await fetch(${JSON.stringify(endpoint)}, {\n  method: "POST",\n  headers: { "Content-Type": "application/json" },\n  body: JSON.stringify({\n    jsonrpc: "2.0", method: "eth_chainId",\n    params: [], id: 1\n  })\n});\nconsole.log(await response.json());`,
    Python: `import json\nfrom urllib.request import Request, urlopen\n\nrequest = Request(\n    ${JSON.stringify(endpoint)},\n    data=json.dumps(${payload}).encode(),\n    headers={"Content-Type": "application/json"}\n)\nprint(json.load(urlopen(request)))`,
  }
  function validateEndpoint() {
    if (input.current && !input.current.validity.valid) {
      setInteracted(true)
      setValidation(input.current.validationMessage)
      input.current.focus()
      return false
    }
    return true
  }
  return <section className="quickstart" id="first-request" aria-labelledby="quickstart-title">
    <div className="quickstart-copy"><p className="eyebrow">YOUR FIRST INTEGRATION</p><h2 id="quickstart-title">Choose your starting point.<br />Connect the whole network.</h2><p>Start with payments, agent automation, deterministic programs or an EVM application. Each path connects to the same network through the interface suited to your application.</p><nav aria-label="Developer quickstarts"><p><Link href="/payments-quickstart" className="text-link">Payments: fund an account and send value →</Link></p><p><Link href="/agents" className="text-link">Agents: identity, authority and commerce →</Link></p><p><Link href="/programs" className="text-link">Programs: publish deterministic logic →</Link></p><p><Link href="/xdk" className="text-link">XDK: SDKs, CLI and developer tools →</Link></p><p><Link href="/platform-api" className="text-link">Platform: gateways and application APIs →</Link></p><p><Link href="/json-rpc" className="text-link">EVM: make your first JSON-RPC request →</Link></p></nav><div className="response-note"><span>Expected chain ID</span><code>0x7d <span>/ 125</span></code><small>The hexadecimal value 0x7d corresponds to chain ID 125.</small></div></div>
    <div className="code-workbench">
      <div className="workbench-title"><span className="method-badge">POST</span><span>eth_chainId</span><span className="workbench-label">JSON-RPC</span></div>
      <label className="endpoint-field" htmlFor="rpc-endpoint"><span>YOUR RPC ENDPOINT</span><input ref={input} id="rpc-endpoint" name="rpc-endpoint" autoComplete="url" type="url" required value={endpoint} onChange={e => { setEndpoint(e.target.value); if (interacted) setValidation(e.target.validationMessage) }} onBlur={e => { setInteracted(true); setValidation(e.target.validationMessage) }} spellCheck={false} aria-label="RPC endpoint" aria-invalid={!!validation} aria-describedby={validation ? 'endpoint-error' : undefined} /></label>
      {validation && <p className="field-error" id="endpoint-error" role="alert">{validation}</p>}
      <div className="code-toolbar"><div className="code-tabs" role="tablist" aria-label="Request language">{(['cURL', 'JavaScript', 'Python'] as Language[]).map(item => <button type="button" key={item} role="tab" id={`tab-${item}`} aria-selected={language === item} aria-controls="request-code" tabIndex={language === item ? 0 : -1} onClick={() => { setLanguage(item) }} onKeyDown={e => {
        const languages: Language[] = ['cURL', 'JavaScript', 'Python']
        if (['ArrowRight', 'ArrowLeft', 'Home', 'End'].includes(e.key)) {
          e.preventDefault()
          const next = e.key === 'Home' ? languages[0] : e.key === 'End' ? languages[2] : languages[(languages.indexOf(item) + (e.key === 'ArrowRight' ? 1 : 2)) % 3]
          setLanguage(next)
          const tab = document.getElementById(`tab-${next}`)
          tab?.focus({ preventScroll: true })
          tab?.scrollIntoView({ block: 'nearest', inline: 'nearest' })
        }
      }}>{item}</button>)}</div><CopyControl targetId="request-code" label="Copy code" className="copy-snippet" icon={false} resetKey={language + endpoint} beforeCopy={validateEndpoint} onFeedback={setCopyFeedback} errorMessage="Copy unavailable. Select the code below to copy it." /></div>
      <pre id="request-code" role="tabpanel" aria-labelledby={`tab-${language}`} tabIndex={0} data-copy-managed="true"><code>{snippets[language]}</code></pre>
      <div className="workbench-footer"><span>Run against your own node</span><span><span hidden={!!copyFeedback}>No API key in this example</span><span role="status">{copyFeedback}</span></span></div>
    </div>
  </section>
}
