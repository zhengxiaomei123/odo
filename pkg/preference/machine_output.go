package preference

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	prefAPIVersion = "odo.dev/v1alpha1"
	prefKind       = "PreferenceList"
)

type PreferenceList struct {
	metav1.TypeMeta `json:",inline"`
	Items           []PreferenceItem `json:"items,omitempty"`
}

type PreferenceItem struct {
	Name        string
	Value       interface{} // The value set by the user, this will be nil if the user hasn't set it
	Default     interface{} // default value of the preference if the user hasn't set the value
	Type        string      // the type of the preference, possible values int, string, bool
	Description string      // The description of the preference
}

func NewPreferenceList(prefInfo PreferenceInfo) PreferenceList {
	return PreferenceList{
		TypeMeta: metav1.TypeMeta{
			APIVersion: prefAPIVersion,
			Kind:       prefKind,
		},
		Items: toPreferenceItems(prefInfo),
	}
}

func toPreferenceItems(prefInfo PreferenceInfo) []PreferenceItem {
	odoSettings := prefInfo.OdoSettings
	return []PreferenceItem{
		{
			Name:        UpdateNotificationSetting,
			Value:       odoSettings.UpdateNotification,
			Default:     true,
			Type:        getType(prefInfo.GetUpdateNotification()), // use the Getter here to determine type
			Description: UpdateNotificationSettingDescription,
		},
		{
			Name:        NamePrefixSetting,
			Value:       odoSettings.NamePrefix,
			Default:     "",
			Type:        getType(prefInfo.GetNamePrefix()),
			Description: NamePrefixSettingDescription,
		},
		{
			Name:        TimeoutSetting,
			Value:       odoSettings.Timeout,
			Default:     DefaultTimeout,
			Type:        getType(prefInfo.GetTimeout()),
			Description: TimeoutSettingDescription,
		},
		{
			Name:        BuildTimeoutSetting,
			Value:       odoSettings.BuildTimeout,
			Default:     DefaultBuildTimeout,
			Type:        getType(prefInfo.GetBuildTimeout()),
			Description: BuildTimeoutSettingDescription,
		},
		{
			Name:        PushTimeoutSetting,
			Value:       odoSettings.PushTimeout,
			Default:     DefaultPushTimeout,
			Type:        getType(prefInfo.GetPushTimeout()),
			Description: PushTimeoutSettingDescription,
		},
		{
			Name:        ExperimentalSetting,
			Value:       odoSettings.Experimental,
			Default:     false,
			Type:        getType(prefInfo.GetExperimental()),
			Description: ExperimentalDescription,
		},
		{
			Name:        PushTargetSetting,
			Value:       odoSettings.PushTarget,
			Default:     KubePushTarget,
			Type:        getType(prefInfo.GetPushTarget()),
			Description: PushTargetDescription,
		},
	}
}

func getType(v interface{}) string {

	rv := reflect.ValueOf(v)

	if rv.Kind() == reflect.Ptr {
		return rv.Elem().Kind().String()
	}

	return rv.Kind().String()
}
