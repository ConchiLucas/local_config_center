package catalog

var antigravityImageAspects = []PickerOption{
	{ID: "", Label: "默认"},
	{ID: "1x1", Label: "1:1"},
	{ID: "3x4", Label: "3:4"},
	{ID: "4x3", Label: "4:3"},
	{ID: "16x9", Label: "16:9"},
	{ID: "9x16", Label: "9:16"},
	{ID: "21x9", Label: "21:9"},
}

var antigravityImageResolutions = []PickerOption{
	{ID: "", Label: "默认"},
	{ID: "2k", Label: "2K"},
	{ID: "4k", Label: "4K"},
}

func ComposeAntigravityImageModel(family, resolution, aspect string) string {
	if family != "pro-3" {
		return "gemini-3.1-flash-image"
	}
	slug := "gemini-3-pro-image"
	if resolution != "" {
		slug += "-" + resolution
	}
	if aspect != "" {
		slug += "-" + aspect
	}
	return slug
}

func AntigravityImageModels() []ModelOption {
	out := []ModelOption{{
		Category: "Gemini 3.1 Flash Image",
		Model:    "gemini-3.1-flash-image",
		Label:    "gemini-3.1-flash-image",
		Note:     "默认生图",
	}}
	for _, res := range antigravityImageResolutions {
		cat := "Gemini 3 Pro Image"
		if res.ID == "2k" {
			cat = "Gemini 3 Pro Image · 2K"
		} else if res.ID == "4k" {
			cat = "Gemini 3 Pro Image · 4K"
		}
		for _, aspect := range antigravityImageAspects {
			slug := ComposeAntigravityImageModel("pro-3", res.ID, aspect.ID)
			note := res.Label
			if aspect.ID != "" {
				if note == "默认" {
					note = aspect.Label
				} else {
					note = res.Label + " · " + aspect.Label
				}
			} else if res.ID == "" {
				note = "默认"
			}
			out = append(out, ModelOption{Category: cat, Model: slug, Label: slug, Note: note})
		}
	}
	return out
}

func AntigravityImagePicker() ModelPicker {
	return ModelPicker{
		Style: "antigravity-image",
		Families: []ModelFamilySpec{
			{ID: "flash-3.1", Label: "Gemini 3.1 Flash Image", Model: "gemini-3.1-flash-image"},
			{
				ID:          "pro-3",
				Label:       "Gemini 3 Pro Image",
				Base:        "gemini-3-pro-image",
				Resolutions: antigravityImageResolutions,
				Aspects:     antigravityImageAspects,
			},
		},
	}
}

func applyAntigravityImageDefaults(cfg PublicConfig, family Family, kind string) PublicConfig {
	if family.ID != "antigravity" || kind != "image" {
		return cfg
	}
	known := AntigravityImageModels()
	bySlug := map[string]ModelOption{}
	for _, item := range cfg.Models {
		bySlug[item.Model] = item
	}
	out := make([]ModelOption, 0, len(known)+len(cfg.Models))
	seen := map[string]bool{}
	for _, item := range known {
		if existing, ok := bySlug[item.Model]; ok {
			item.ProviderID = existing.ProviderID
			item.Registered = true
		}
		out = append(out, item)
		seen[item.Model] = true
	}
	for _, item := range cfg.Models {
		if seen[item.Model] {
			continue
		}
		item.Category = "配置中心已登记"
		item.Registered = true
		out = append(out, item)
	}
	cfg.Models = out
	cfg.ModelPicker = AntigravityImagePicker()
	cfg.AllowCustomModel = true
	if cfg.Model == "" {
		cfg.Model = "gemini-3.1-flash-image"
	}
	return cfg
}
