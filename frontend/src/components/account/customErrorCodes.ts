export interface BuildCustomErrorCodesResult {
  codes: number[]
  invalid: boolean
}

export const isValidCustomErrorCode = (code: unknown): code is number =>
  typeof code === 'number' &&
  Number.isInteger(code) &&
  code >= 100 &&
  code <= 599

export const buildCustomErrorCodesResult = (codes: unknown[]): BuildCustomErrorCodesResult => {
  const out: number[] = []
  let invalid = false

  for (const code of codes) {
    if (!isValidCustomErrorCode(code)) {
      invalid = true
      continue
    }
    out.push(code)
  }

  return { codes: out, invalid }
}
