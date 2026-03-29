export const BUCKET_NAME_MAX_LENGTH = 50
export const BUCKET_NAME_SPACE_WARNING = "Bucket names cannot contain spaces."
export const BUCKET_NAME_LENGTH_WARNING = `Bucket names must be ${BUCKET_NAME_MAX_LENGTH} characters or fewer.`

export const sanitizeBucketName = (value: string) => value.replace(/\s+/g, "").slice(0, BUCKET_NAME_MAX_LENGTH)

export const isValidBucketName = (value: string) =>
  value.length > 0 &&
  value.length <= BUCKET_NAME_MAX_LENGTH &&
  !/\s/.test(value)

export const getBucketNameWarnings = (value: string) => {
  const warnings: string[] = []

  if (/\s/.test(value)) {
    warnings.push(BUCKET_NAME_SPACE_WARNING)
  }

  if (value.replace(/\s+/g, "").length > BUCKET_NAME_MAX_LENGTH) {
    warnings.push(BUCKET_NAME_LENGTH_WARNING)
  }

  return warnings
}
