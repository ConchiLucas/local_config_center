package catalog

func AttachGateway(cat Catalog, token string) Catalog {
	if token == "" {
		token = "local-gateway"
	}
	family := localFamily()
	idx := -1
	for i := range cat.Groups {
		if cat.Groups[i].ID == family.ID {
			idx = i
			break
		}
	}
	if idx < 0 {
		cat.Groups = append(cat.Groups, Group{
			ID:      family.ID,
			Label:   family.Label,
			Eyebrow: family.Eyebrow,
			Mark:    family.Mark,
			Icon:    family.Icon,
		})
		idx = len(cat.Groups) - 1
	}
	addConfig(&cat.Groups[idx], gatewayEntry(token))
	return cat
}

func gatewayEntry(token string) PublicConfig {
	return finish(PublicConfig{
		ID:          "gateway:local",
		GroupID:     "local",
		PageID:      "gateway",
		Kind:        "gateway",
		Title:       "内部调用网关",
		Model:       "/v1/invoke",
		Revision:    "gateway",
		RendererKey: "gateway",
		PublicProperties: []Property{
			{Label: "调用地址", Value: "/v1/invoke"},
			{Label: "模型清单", Value: "/v1/models"},
			{Label: "JSON 鉴权字段", Value: "auth"},
			{Label: "鉴权密钥", Value: token},
			{Label: "环境变量", Value: "GATEWAY_AUTH"},
		},
		CredentialState: "configured",
		Status:          "gateway",
		Capabilities: CapabilitySet{
			Delivery:     []string{"complete"},
			Formats:      []string{"json"},
			Cancellation: "unsupported",
		},
	}, nil)
}
