package app

import (
	"time"

	entryMessage "github.com/jasonblanchard/di-velocity/src/di_messages/entry"
	"github.com/jasonblanchard/di-velocity/src/di_messages/insights"
	insightsMessage "github.com/jasonblanchard/di-velocity/src/di_messages/insights"
	"github.com/jasonblanchard/di-velocity/src/op"
	"github.com/jasonblanchard/di-velocity/src/utils"
	"github.com/nats-io/nats.go"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

// Handlers configures all the handlers
func (service *Service) Handlers() {
	if service.TestMode == true {
		service.RegisterHandler(service.WithResponse(service.HandleDrop()))
	}
	service.RegisterHandler(service.HandleEntryUpdated())
	service.RegisterHandler(service.HandleIncrementDailyCounter())
	service.RegisterHandler(service.WithResponse(service.handleGetVelocity()))
}

// HandleDrop insights.store.drop
func (service *Service) HandleDrop() (string, MsgHandler) {
	return "insights.store.drop", func(m *nats.Msg) ([]byte, error) {
		err := op.DropDailyCounts(service.Store)
		if err != nil {
			return nil, errors.Wrap(err, "DropDailyCounts failed")
		}

		return []byte(""), nil
	}
}

// HandleEntryUpdated info.entry.updated
func (service *Service) HandleEntryUpdated() (string, MsgHandler) {
	return "info.entry.updated", func(m *nats.Msg) ([]byte, error) {
		entryUpdatedMessage := &entryMessage.InfoEntryUpdated{}
		err := proto.Unmarshal(m.Data, entryUpdatedMessage)
		if err != nil {
			return nil, errors.Wrap(err, "Unmarshall failed")
		}

		normalizedDay := utils.NormalizeTime(time.Unix(entryUpdatedMessage.Payload.UpdatedAt.Seconds, 0))
		day := utils.TimeToProtoTime(normalizedDay)

		incrementDailyCounterRequest := &insightsMessage.IncrementDailyCounter{
			Payload: &insightsMessage.IncrementDailyCounter_Payload{
				Day:       &day,
				CreatorId: entryUpdatedMessage.Payload.CreatorId,
			},
		}

		request, err := proto.Marshal(incrementDailyCounterRequest)

		if err != nil {
			return nil, errors.Wrap(err, "Marshal failed")
		}

		service.Broker.Publish("insights.increment.dailyCounter", request)

		return nil, nil
	}
}

// HandleIncrementDailyCounter insights.increment.dailyCounter
func (service *Service) HandleIncrementDailyCounter() (string, MsgHandler) {
	return "insights.increment.dailyCounter", func(m *nats.Msg) ([]byte, error) {
		requestMessage := &insights.IncrementDailyCounter{}
		err := proto.Unmarshal(m.Data, requestMessage)
		if err != nil {
			return nil, errors.Wrap(err, "unmarshall failed")
		}

		day := time.Unix(requestMessage.Payload.Day.Seconds, 0).UTC()

		err = op.IncrementDailyCounter(service.Store, day, requestMessage.Payload.CreatorId)
		if err != nil {
			return nil, errors.Wrap(err, "increment failed")
		}

		return nil, nil
	}
}

// handleGetVelocity insights.get.velocity
func (service *Service) handleGetVelocity() (string, MsgHandler) {
	return "insights.get.velocity", func(m *nats.Msg) ([]byte, error) {
		requestMessage := &insightsMessage.GetVelocityRequest{}
		err := proto.Unmarshal(m.Data, requestMessage)
		if err != nil {
			// TODO: Respond with error type
			return nil, errors.Wrap(err, "unmarshall failed")
		}

		normalizedStart := utils.NormalizeTime(time.Unix(requestMessage.Payload.Start.Seconds, 0).UTC())
		normalizedEnd := utils.NormalizeTime(time.Unix(requestMessage.Payload.End.Seconds, 0).UTC())

		dailyCounts, err := op.GetDailyCounts(service.Store, normalizedStart, normalizedEnd)
		if err != nil {
			return nil, errors.Wrap(err, "get daily counts failed")
		}

		dailyVelocities := dailyCounts.ToVelocityScores()

		responseMessage := &insightsMessage.GetVelocityResponse{
			Payload: dailyVelocities.ToDtoPayload(),
		}

		return proto.Marshal(responseMessage)
	}
}
