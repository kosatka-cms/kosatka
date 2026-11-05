package entity

// FieldUploadPolicy is the effective upload-validation policy for one
// image/file field. A field's own settings are NOT clamped to the sitewide
// UploadSettings: a value set on the field wins even if wider; sitewide
// values only fill axes the field leaves unset (see ResolveFieldUploadPolicy).
type FieldUploadPolicy struct {
	// AllowedExtensions - lowercase, no leading dot ("jpg", "pdf"). nil for
	// an Image field means "use the sitewide MIME allow-list"; File fields
	// always resolve to a non-nil list.
	AllowedExtensions []string
	MaxFileSizeBytes  int
	// MaxWidth/MaxHeight - 0,0 means the sitewide MaxImageDimension as a
	// square cap. Unused for File fields.
	MaxWidth, MaxHeight int
	// MinWidth/MinHeight - 0,0 means no minimum. Unused for File fields.
	MinWidth, MinHeight int
}

// defaultFileExtensions - allow-list for a File field without its own
// extensions (the sitewide policy is image-only).
var defaultFileExtensions = []string{"pdf", "doc", "docx", "xls", "xlsx", "zip", "txt"}

// ResolveFieldUploadPolicy builds the effective policy for one field, axis
// by axis: the field's own setting, else the sitewide default. File fields
// fall back to defaultFileExtensions and get no resolution limits.
func ResolveFieldUploadPolicy(field FieldInstanceDef, sitewide UploadSettings) FieldUploadPolicy {
	policy := FieldUploadPolicy{
		AllowedExtensions: field.AllowedFileExtensions(),
		MaxFileSizeBytes:  field.MaxUploadSizeBytes(),
	}
	if policy.MaxFileSizeBytes == 0 {
		policy.MaxFileSizeBytes = sitewide.MaxFileSizeBytes
	}

	if field.FieldType == FieldTypeFile && len(policy.AllowedExtensions) == 0 {
		policy.AllowedExtensions = defaultFileExtensions
	}

	if field.FieldType == FieldTypeImage {
		policy.MaxWidth, policy.MaxHeight = field.MaxResolution()
		if policy.MaxWidth == 0 && policy.MaxHeight == 0 {
			policy.MaxWidth, policy.MaxHeight = sitewide.MaxImageDimension, sitewide.MaxImageDimension
		}
		policy.MinWidth, policy.MinHeight = field.MinResolution()
	}

	return policy
}

// ResolveAvatarUploadPolicy builds the upload policy for user avatars. With
// no AvatarAllowedExtensions, AllowedExtensions stays nil and the caller
// uses the sitewide MIME check; an explicit list replaces that check.
// MaxFileSizeBytes falls back to sitewide. The resize target is the
// MaxAvatarDimension constant, not part of this policy.
func ResolveAvatarUploadPolicy(account AccountSettings, sitewide UploadSettings) FieldUploadPolicy {
	policy := FieldUploadPolicy{
		AllowedExtensions: account.AvatarAllowedExtensions,
		MaxFileSizeBytes:  account.AvatarMaxFileSizeBytes,
	}
	if policy.MaxFileSizeBytes == 0 {
		policy.MaxFileSizeBytes = sitewide.MaxFileSizeBytes
	}
	return policy
}

// ResolveTextFormatUploadPolicy builds the upload policy for editor
// (TinyMCE file manager) image uploads of a text format: nil extensions
// fall through to the sitewide MIME check, an explicit list replaces it;
// MaxFileSizeBytes falls back to sitewide; an image larger than the
// sitewide MaxImageDimension is scaled down.
func ResolveTextFormatUploadPolicy(format TextFormatDef, sitewide UploadSettings) FieldUploadPolicy {
	policy := FieldUploadPolicy{
		AllowedExtensions: format.AllowedExtensionsList(),
		MaxFileSizeBytes:  format.MaxFileSizeBytes,
		MaxWidth:          sitewide.MaxImageDimension,
		MaxHeight:         sitewide.MaxImageDimension,
	}
	if policy.MaxFileSizeBytes == 0 {
		policy.MaxFileSizeBytes = sitewide.MaxFileSizeBytes
	}
	return policy
}
