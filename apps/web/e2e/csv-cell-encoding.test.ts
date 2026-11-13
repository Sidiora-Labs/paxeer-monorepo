import assert from "node:assert/strict";
import test from "node:test";

import { createCsvBlob } from "../packages/layerx-ui/src/lib/format.ts";

/*
 * Task 22.33.507 / audit 22.30.151: csv-formula-injection is proven by
 * csvFormulaPrefixesRemainText, csvQuotesAndCarriageReturnsStayInTheirCells,
 * and csvTextAndNumericAmountsKeepTheirTypes below. Each consumes the actual
 * download codec's Blob through a real object URL and fetch, without a DOM double.
 * MoneyList's existing doExport calls downloadCsv, which uses this codec for
 * every header and cell; no out-of-scope wiring change is needed.
 * Baseline verification was not run: the task's ordered instructions prohibit
 * gates before implementation and verification outside hf-done. No prior gate
 * ledger or baseline failures existed when this attempt began.
 */

async function consumeCsv(header: string[], rows: (string | number)[][]): Promise<string> {
  const blob = createCsvBlob(header, rows);
  const url = URL.createObjectURL(blob);
  try {
    const response = await fetch(url);
    assert.equal(response.status, 200);
    assert.equal(response.headers.get("content-type"), "text/csv;charset=utf-8");
    return await response.text();
  } finally {
    URL.revokeObjectURL(url);
  }
}

async function csvFormulaPrefixesRemainText() {
  const cases = [
    ["=1+1", "'=1+1"],
    ["+SUM(A1:A2)", "'+SUM(A1:A2)"],
    ["-1+2", "'-1+2"],
    ["@SUM(A1:A2)", "'@SUM(A1:A2)"],
    ['=HYPERLINK("https://example.invalid","open")', '"\'=HYPERLINK(""https://example.invalid"",""open"")"'],
    [" =1+1", "' =1+1"],
    ["\t=1+1", "'\t=1+1"],
    ["\tlabel", "'\tlabel"],
    ["\r=1+1", '"\'\r=1+1"'],
    ["\n=1+1", '"\'\n=1+1"'],
    [" \t\r\n@SUM(A1:A2)", '"\' \t\r\n@SUM(A1:A2)"'],
    ["\uFEFF+1+1", "'\uFEFF+1+1"],
  ] as const;

  for (const [input, encoded] of cases) {
    const csv = await consumeCsv([input, "Title"], [[input, "literal"]]);
    assert.equal(csv, `${encoded},Title\n${encoded},literal`, JSON.stringify(input));
  }
}

async function csvQuotesAndCarriageReturnsStayInTheirCells() {
  const csv = await consumeCsv(
    ["Group", "Title", "Subtitle", "Status", "Date", "Amount", "Currency"],
    [
      ["November, 2026", 'Payment "one"', "first\rsecond", "done", "2026-11-04", "12.50", "USD"],
      ["November", "line\nbreak", "line\r\nbreak", "", "2026-11-04", "0.00", ""],
      ["November", "\r=1+1", "line\r=1+1", "", "2026-11-04", "2.00", "USD"],
    ],
  );
  assert.equal(csv, [
    "Group,Title,Subtitle,Status,Date,Amount,Currency",
    '"November, 2026","Payment ""one""","first\rsecond",done,2026-11-04,12.50,USD',
    'November,"line\nbreak","line\r\nbreak",,2026-11-04,0.00,',
    'November,"\'\r=1+1","line\r=1+1",,2026-11-04,2.00,USD',
  ].join("\n"));
}

async function csvTextAndNumericAmountsKeepTheirTypes() {
  const csv = await consumeCsv(
    ["Number", "Text", "Label", "Empty"],
    [
      [-12.5, "-12.50", "Ordinary payment", ""],
      [12.5, "+12.50", "O'Brien", ""],
      [0, "00125", "2026-11-04", ""],
      [1000, "1,000.00", "already '=literal", ""],
    ],
  );
  assert.equal(csv, [
    "Number,Text,Label,Empty",
    "-12.5,'-12.50,Ordinary payment,",
    "12.5,'+12.50,O'Brien,",
    "0,00125,2026-11-04,",
    '1000,"1,000.00",already \'=literal,',
  ].join("\n"));
  assert.equal(await consumeCsv([], []), "");
}

test(csvFormulaPrefixesRemainText);
test(csvQuotesAndCarriageReturnsStayInTheirCells);
test(csvTextAndNumericAmountsKeepTheirTypes);
