package fsm

// FSM States for the bot
//
// State Transitions for Forum Post Manager:
//
// Post Creation Flow:
//   StateAdminMenu -> StateNewPostSelectType (via /new command)
//   StateNewPostSelectType -> StateNewPostEnterText (via type selection)
//   StateNewPostEnterText -> StateNewPostConfirm (via text input)
//   StateNewPostConfirm -> StateAdminMenu (via confirmation or /cancel)
//
// Post Editing Flow:
//   StateAdminMenu -> StateEditPostEnterLink (via /edit command)
//   StateEditPostEnterLink -> StateEditPostEnterText (via valid link)
//   StateEditPostEnterText -> StateAdminMenu (via text input or /cancel)
//
// Post Deletion Flow:
//   StateAdminMenu -> StateDeletePostEnterLink (via /delete command)
//   StateDeletePostEnterLink -> StateAdminMenu (via link input or /cancel)
//
// Type Creation Flow:
//   StateAdminMenu -> StateNewTypeEnterName (via settings -> new type)
//   StateNewTypeEnterName -> StateNewTypeEnterImage (via name input)
//   StateNewTypeEnterImage -> StateNewTypeEnterTemplate (via image or skip)
//   StateNewTypeEnterTemplate -> StateAdminMenu (via template input or /cancel)
//
// Type Management Flow:
//   StateAdminMenu -> StateManageTypes (via settings -> manage types)
//   StateManageTypes -> StateEditTypeName/StateEditTypeImage/StateEditTypeTemplate (via type selection)
//   StateEditType* -> StateManageTypes (via input or /cancel)
//
// Access Settings Flow:
//   StateAdminMenu -> StateAccessSettings (via settings -> access settings)
//   StateAccessSettings -> StateEditAdminIDs/StateEditForumID/StateEditTopicID (via setting selection)
//   StateEdit* -> StateAccessSettings (via input or /cancel)
//
// Cancel Command:
//   Any state -> StateAdminMenu (via /cancel command)

const (
	StateInit           = "init"
	StateWelcome        = "welcome"
	StateAwaitingAnswer = "awaiting_answer"
	StateWaitingReview  = "waiting_review"
	StateCompleted      = "completed"

	StateAdminMenu          = "admin_menu"
	StateAdminAddStep       = "admin_add_step"
	StateAdminEditStep      = "admin_edit_step"
	StateAdminManageAnswers = "admin_manage_answers"
	StateAdminEditSettings  = "admin_edit_settings"

	StateAdminAddStepText                = "admin_add_step_text"
	StateAdminAddStepType                = "admin_add_step_type"
	StateAdminAddStepImages              = "admin_add_step_images"
	StateAdminAddCorrectImage            = "admin_add_correct_image"
	StateAdminReplaceCorrectImage        = "admin_replace_correct_image"
	StateAdminDeleteCorrectImage         = "admin_delete_correct_image"
	StateAdminAddStepAnswers             = "admin_add_step_answers"
	StateAdminEditStepText               = "admin_edit_step_text"
	StateAdminEditStepImages             = "admin_edit_step_images"
	StateAdminManageImages               = "admin_manage_images"
	StateAdminAddImage                   = "admin_add_image"
	StateAdminReplaceImage               = "admin_replace_image"
	StateAdminDeleteImage                = "admin_delete_image"
	StateAdminAddAnswer                  = "admin_add_answer"
	StateAdminDeleteAnswer               = "admin_delete_answer"
	StateAdminEditSettingValue           = "admin_edit_setting_value"
	StateAdminUserList                   = "admin_user_list"
	StateAdminUserDetails                = "admin_user_details"
	StateAdminAddHintText                = "admin_add_hint_text"
	StateAdminAddHintImage               = "admin_add_hint_image"
	StateAdminEditHintText               = "admin_edit_hint_text"
	StateAdminEditHintImage              = "admin_edit_hint_image"
	StateAdminBackup                     = "admin_backup"
	StateAdminSendMessage                = "admin_send_message"
	StateAdminEnableGroupRestrictionID   = "admin_enable_group_restriction_id"
	StateAdminEnableGroupRestrictionLink = "admin_enable_group_restriction_link"
	StateAdminEditGroupID                = "admin_edit_group_id"
	StateAdminEditGroupLink              = "admin_edit_group_link"

	// Reply States
	StateReplyEnterLink     = "reply_enter_link"
	StateReplyEnterText     = "reply_enter_text"
	StateReplyConfirm       = "reply_confirm"
	StateEditReplyEnterText = "edit_reply_enter_text"

	// Forum Post Manager States
	StateNewPostSelectType    = "new_post_select_type"
	StateNewPostEnterText     = "new_post_enter_text"
	StateNewPostConfirm       = "new_post_confirm"
	StateEditPostEnterLink    = "edit_post_enter_link"
	StateEditPostEnterText    = "edit_post_enter_text"
	StateDeletePostEnterLink  = "delete_post_enter_link"
	StateNewTypeEnterName     = "new_type_enter_name"
	StateNewTypeEnterEmoji    = "new_type_enter_emoji"
	StateNewTypeEnterImage    = "new_type_enter_image"
	StateNewTypeEnterTemplate = "new_type_enter_template"
	StateManageTypes          = "manage_types"
	StateEditTypeName         = "edit_type_name"
	StateEditTypeEmoji        = "edit_type_emoji"
	StateEditTypeImage        = "edit_type_image"
	StateEditTypeTemplate     = "edit_type_template"
	StateAccessSettings       = "access_settings"
	StateEditAdminIDs         = "edit_admin_ids"
	StateEditForumID          = "edit_forum_id"
	StateEditTopicID          = "edit_topic_id"
)
