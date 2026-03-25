export const CONTAINER_NAME_MAX_LENGTH = 50
export const CONTAINER_NAME_SPACE_WARNING = "Container names cannot contain spaces."
export const CONTAINER_NAME_LENGTH_WARNING = `Container names must be ${CONTAINER_NAME_MAX_LENGTH} characters or fewer.`

export const sanitizeContainerName = (value: string) => value.replace(/\s+/g, "").slice(0, CONTAINER_NAME_MAX_LENGTH)

export const isValidContainerName = (value: string) =>
  value.length > 0 &&
  value.length <= CONTAINER_NAME_MAX_LENGTH &&
  !/\s/.test(value)

export const getContainerNameWarnings = (value: string) => {
  const warnings: string[] = []

  if (/\s/.test(value)) {
    warnings.push(CONTAINER_NAME_SPACE_WARNING)
  }

  if (value.replace(/\s+/g, "").length > CONTAINER_NAME_MAX_LENGTH) {
    warnings.push(CONTAINER_NAME_LENGTH_WARNING)
  }

  return warnings
}
