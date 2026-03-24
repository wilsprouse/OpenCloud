export const FUNCTION_NAME_MAX_LENGTH = 50
export const FUNCTION_NAME_SPACE_WARNING = "Function names cannot contain spaces."
export const FUNCTION_NAME_LENGTH_WARNING = `Function names must be ${FUNCTION_NAME_MAX_LENGTH} characters or fewer.`

export const sanitizeFunctionName = (value: string) => value.replace(/\s+/g, "").slice(0, FUNCTION_NAME_MAX_LENGTH)

export const isValidFunctionName = (value: string) =>
  value.length > 0 &&
  value.length <= FUNCTION_NAME_MAX_LENGTH &&
  !/\s/.test(value)

export const getFunctionNameWarnings = (value: string) => {
  const warnings: string[] = []

  if (/\s/.test(value)) {
    warnings.push(FUNCTION_NAME_SPACE_WARNING)
  }

  if (value.replace(/\s+/g, "").length > FUNCTION_NAME_MAX_LENGTH) {
    warnings.push(FUNCTION_NAME_LENGTH_WARNING)
  }

  return warnings
}
