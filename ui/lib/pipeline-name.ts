export const PIPELINE_NAME_MAX_LENGTH = 50
export const PIPELINE_NAME_SPACE_WARNING = "Pipeline names cannot contain spaces."
export const PIPELINE_NAME_LENGTH_WARNING = `Pipeline names must be ${PIPELINE_NAME_MAX_LENGTH} characters or fewer.`

export const sanitizePipelineName = (value: string) => value.replace(/\s+/g, "").slice(0, PIPELINE_NAME_MAX_LENGTH)

export const isValidPipelineName = (value: string) =>
  value.length > 0 &&
  value.length <= PIPELINE_NAME_MAX_LENGTH &&
  !/\s/.test(value)

export const getPipelineNameWarnings = (value: string) => {
  const warnings: string[] = []

  if (/\s/.test(value)) {
    warnings.push(PIPELINE_NAME_SPACE_WARNING)
  }

  if (value.replace(/\s+/g, "").length > PIPELINE_NAME_MAX_LENGTH) {
    warnings.push(PIPELINE_NAME_LENGTH_WARNING)
  }

  return warnings
}
