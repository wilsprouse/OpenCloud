export const FUNCTION_NAME_MAX_LENGTH = 50

export const sanitizeFunctionName = (value: string) => value.replace(/\s+/g, "").slice(0, FUNCTION_NAME_MAX_LENGTH)

export const isValidFunctionName = (value: string) =>
  value.length > 0 &&
  value.length <= FUNCTION_NAME_MAX_LENGTH &&
  !/\s/.test(value)
