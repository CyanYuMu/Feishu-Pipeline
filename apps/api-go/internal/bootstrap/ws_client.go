package bootstrap

import (
	"context"
	"encoding/json"
	"log"

	"feishu-pipeline/apps/api-go/internal/external/feishu"
	"feishu-pipeline/apps/api-go/internal/service"

	larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/larksuite/oapi-sdk-go/v3/ws"
)

// StartFeishuWSClient 启动飞书 WebSocket 长连接客户端
// 通过长连接接收卡片回调，无需公网回调地址
func StartFeishuWSClient(ctx context.Context, feishuClient *feishu.Client, authService *service.AuthService, pipelineService *service.PipelineService) {
	if !feishuClient.Enabled() {
		log.Printf("[Feishu WS] disabled: feishu client not enabled")
		return
	}

	// 创建事件分发器，监听机器人消息事件与卡片回传交互回调
	evtDispatcher := dispatcher.NewEventDispatcher("", "")

	evtDispatcher.OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
		if pipelineService == nil {
			log.Printf("[Feishu WS] message received but pipeline service is not available")
			return nil
		}
		req := mapP2MessageReceiveV1ToBotEvent(event)
		log.Printf(
			"[Feishu WS] message received: event_type=%s message_id=%s chat_id=%s message_type=%s",
			req.Header.EventType,
			req.Event.Message.MessageID,
			req.Event.Message.ChatID,
			req.Event.Message.MessageType,
		)
		result, err := pipelineService.HandleFeishuBotEvent(ctx, req)
		if err != nil {
			log.Printf("[Feishu WS] handle bot message failed: %v", err)
			return nil
		}
		log.Printf(
			"[Feishu WS] bot message handled: ignored=%t run_id=%s status=%s stage=%s message=%s",
			result.Ignored,
			result.PipelineRunID,
			result.Status,
			result.CurrentStageKey,
			result.Message,
		)
		return nil
	})

	// 使用 OnCustomizedEvent 监听 card.action.trigger 事件
	evtDispatcher.OnCustomizedEvent("card.action.trigger", func(ctx context.Context, req *larkevent.EventReq) error {
		// 解析事件体
		var eventData map[string]interface{}
		if err := json.Unmarshal(req.Body, &eventData); err != nil {
			log.Printf("[Feishu WS] failed to parse event body: %v", err)
			return nil
		}

		log.Printf("[Feishu WS] card action trigger received: %+v", eventData)

		// 提取关键信息
		action, _ := eventData["action"].(map[string]interface{})
		if action == nil {
			return nil
		}

		input, _ := action["input"].(map[string]interface{})
		intent, _ := input["intent"].(string)
		value, _ := action["value"].(map[string]interface{})
		if intent == "" {
			intent = getStringFromMap(value, "intent")
		}

		message, _ := eventData["message"].(map[string]interface{})
		sender, _ := message["sender"].(map[string]interface{})
		senderID, _ := sender["sender_id"].(map[string]interface{})
		openID, _ := senderID["open_id"].(string)

		messageID, _ := message["message_id"].(string)

		log.Printf("[Feishu WS] intent=%s open_id=%s message_id=%s", intent, openID, messageID)

		if pipelineService != nil && (intent == "pipeline_checkpoint_approve" || intent == "pipeline_checkpoint_reject") {
			checkpointID := getStringFromMap(input, "checkpointId")
			if checkpointID == "" {
				checkpointID = getStringFromMap(value, "checkpointId")
			}
			switch intent {
			case "pipeline_checkpoint_approve":
				_, err := pipelineService.ApproveCheckpoint(ctx, checkpointID, "通过飞书审批卡片确认", openID)
				if err != nil {
					log.Printf("[Feishu WS] approve pipeline checkpoint failed: %v", err)
				}
			case "pipeline_checkpoint_reject":
				_, err := pipelineService.RejectCheckpoint(ctx, checkpointID, "通过飞书审批卡片驳回，请补充后重做", openID)
				if err != nil {
					log.Printf("[Feishu WS] reject pipeline checkpoint failed: %v", err)
				}
			}
			return nil
		}

		// 调用 authService 处理卡片回调
		_, _ = authService.HandleFeishuCardCallback(ctx, service.FeishuCardCallbackRequest{
			Type: "card",
			Event: struct {
				Action string `json:"action"`
				Input  struct {
					Intent string `json:"intent"`
				} `json:"input"`
				Message struct {
					MessageID  string `json:"message_id"`
					RootID     string `json:"root_id"`
					ParentID   string `json:"parent_id"`
					CreateTime string `json:"create_time"`
					ChatID     string `json:"chat_id"`
					Sender     struct {
						SenderID struct {
							OpenID  string `json:"open_id"`
							UserID  string `json:"user_id"`
							UnionID string `json:"union_id"`
						} `json:"sender_id"`
					} `json:"sender"`
				} `json:"message"`
			}{
				Action: getStringFromMap(action, "action_name"),
				Input: struct {
					Intent string `json:"intent"`
				}{
					Intent: intent,
				},
				Message: struct {
					MessageID  string `json:"message_id"`
					RootID     string `json:"root_id"`
					ParentID   string `json:"parent_id"`
					CreateTime string `json:"create_time"`
					ChatID     string `json:"chat_id"`
					Sender     struct {
						SenderID struct {
							OpenID  string `json:"open_id"`
							UserID  string `json:"user_id"`
							UnionID string `json:"union_id"`
						} `json:"sender_id"`
					} `json:"sender"`
				}{
					MessageID:  messageID,
					RootID:     getStringFromMap(message, "root_id"),
					ParentID:   getStringFromMap(message, "parent_id"),
					CreateTime: getStringFromMap(message, "create_time"),
					ChatID:     getStringFromMap(message, "chat_id"),
					Sender: struct {
						SenderID struct {
							OpenID  string `json:"open_id"`
							UserID  string `json:"user_id"`
							UnionID string `json:"union_id"`
						} `json:"sender_id"`
					}{
						SenderID: struct {
							OpenID  string `json:"open_id"`
							UserID  string `json:"user_id"`
							UnionID string `json:"union_id"`
						}{
							OpenID:  openID,
							UserID:  getStringFromMap(senderID, "user_id"),
							UnionID: getStringFromMap(senderID, "union_id"),
						},
					},
				},
			},
		})

		return nil
	})

	// 创建 WebSocket 客户端
	wsClient := ws.NewClient(
		feishuClient.AppID(),
		feishuClient.AppSecret(),
		ws.WithEventHandler(evtDispatcher),
	)

	// 启动长连接（在 goroutine 中运行）
	go func() {
		log.Printf("[Feishu WS] starting websocket client...")
		if err := wsClient.Start(ctx); err != nil {
			log.Printf("[Feishu WS] start failed: %v", err)
		} else {
			log.Printf("[Feishu WS] websocket client started successfully")
		}
	}()
}

// getStringFromMap 安全地从 map 中获取字符串值
func getStringFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func mapP2MessageReceiveV1ToBotEvent(event *larkim.P2MessageReceiveV1) service.FeishuBotEventRequest {
	var req service.FeishuBotEventRequest
	req.Type = "event_callback"
	req.Header.EventType = "im.message.receive_v1"
	if event == nil || event.Event == nil {
		return req
	}
	if event.EventV2Base != nil && event.EventV2Base.Header != nil {
		req.Header.EventType = event.EventV2Base.Header.EventType
	}
	if event.Event.Sender != nil && event.Event.Sender.SenderId != nil {
		req.Event.Sender.SenderID.OpenID = valueOf(event.Event.Sender.SenderId.OpenId)
		req.Event.Sender.SenderID.UserID = valueOf(event.Event.Sender.SenderId.UserId)
		req.Event.Sender.SenderID.UnionID = valueOf(event.Event.Sender.SenderId.UnionId)
	}
	if event.Event.Message != nil {
		req.Event.Message.MessageID = valueOf(event.Event.Message.MessageId)
		req.Event.Message.ChatID = valueOf(event.Event.Message.ChatId)
		req.Event.Message.MessageType = valueOf(event.Event.Message.MessageType)
		req.Event.Message.Content = valueOf(event.Event.Message.Content)
	}
	return req
}

func valueOf(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
