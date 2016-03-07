// Copyright © 2015 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package broker

import (
	"testing"

	"github.com/TheThingsNetwork/ttn/core"
	"github.com/TheThingsNetwork/ttn/core/mocks"
	"github.com/TheThingsNetwork/ttn/utils/errors"
	errutil "github.com/TheThingsNetwork/ttn/utils/errors/checks"
	"github.com/TheThingsNetwork/ttn/utils/pointer"
	testutil "github.com/TheThingsNetwork/ttn/utils/testing"
	"github.com/brocaar/lorawan"
)

func TestRegister(t *testing.T) {
	{
		testutil.Desc(t, "Register a device")

		// Build
		an := mocks.NewMockAckNacker()
		store := newMockStorage()
		r := mocks.NewMockBRegistration()

		// Operate
		broker := New(store, testutil.GetLogger(t, "Broker"))
		err := broker.Register(r, an)

		// Check
		errutil.CheckErrors(t, nil, err)
		mocks.CheckAcks(t, true, an.InAck)
		CheckRegistrations(t, r, store.InStoreDevices)
		CheckRegistrations(t, nil, store.InStoreApp)
	}

	// -------------------

	{
		testutil.Desc(t, "Register an application")

		// Build
		an := mocks.NewMockAckNacker()
		store := newMockStorage()
		r := mocks.NewMockARegistration()

		// Operate
		broker := New(store, testutil.GetLogger(t, "Broker"))
		err := broker.Register(r, an)

		// Check
		errutil.CheckErrors(t, nil, err)
		mocks.CheckAcks(t, true, an.InAck)
		CheckRegistrations(t, nil, store.InStoreDevices)
		CheckRegistrations(t, r, store.InStoreApp)
	}

	// -------------------

	{
		testutil.Desc(t, "Register a device | store failed")

		// Build
		an := mocks.NewMockAckNacker()
		store := newMockStorage()
		store.Failures["StoreDevice"] = errors.New(errors.Structural, "Mock Error: Store Failed")
		r := mocks.NewMockBRegistration()

		// Operate
		broker := New(store, testutil.GetLogger(t, "Broker"))
		err := broker.Register(r, an)

		// Check
		errutil.CheckErrors(t, pointer.String(string(errors.Structural)), err)
		mocks.CheckAcks(t, false, an.InAck)
		CheckRegistrations(t, r, store.InStoreDevices)
		CheckRegistrations(t, nil, store.InStoreApp)
	}

	// -------------------

	{
		testutil.Desc(t, "Register an application | store failed")

		// Build
		an := mocks.NewMockAckNacker()
		store := newMockStorage()
		store.Failures["StoreApplication"] = errors.New(errors.Structural, "Mock Error: Store Failed")
		r := mocks.NewMockARegistration()

		// Operate
		broker := New(store, testutil.GetLogger(t, "Broker"))
		err := broker.Register(r, an)

		// Check
		errutil.CheckErrors(t, pointer.String(string(errors.Structural)), err)
		mocks.CheckAcks(t, false, an.InAck)
		CheckRegistrations(t, nil, store.InStoreDevices)
		CheckRegistrations(t, r, store.InStoreApp)
	}

	// -------------------

	{
		testutil.Desc(t, "Register an entry | Wrong registration")

		// Build
		an := mocks.NewMockAckNacker()
		store := newMockStorage()
		r := mocks.NewMockRRegistration()

		// Operate
		broker := New(store, testutil.GetLogger(t, "Broker"))
		err := broker.Register(r, an)

		// Check
		errutil.CheckErrors(t, pointer.String(string(errors.Structural)), err)
		mocks.CheckAcks(t, false, an.InAck)
		CheckRegistrations(t, nil, store.InStoreDevices)
		CheckRegistrations(t, nil, store.InStoreApp)
	}
}

func TestHandleUp(t *testing.T) {
	{
		testutil.Desc(t, "Send an unknown packet")

		// Build
		an := mocks.NewMockAckNacker()
		adapter := mocks.NewMockAdapter()
		store := newMockStorage()
		store.Failures["LookupDevices"] = errors.New(errors.Behavioural, "Mock Error: Not Found")
		data, _ := newBPacket(
			[4]byte{2, 3, 2, 3},
			"Payload",
			[16]byte{1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8},
			5,
		).MarshalBinary()

		// Operate
		broker := New(store, testutil.GetLogger(t, "Broker"))
		err := broker.HandleUp(data, an, adapter)

		// Check
		errutil.CheckErrors(t, pointer.String(string(errors.Behavioural)), err)
		mocks.CheckAcks(t, false, an.InAck)
		CheckRegistrations(t, nil, store.InStoreDevices)
		CheckRegistrations(t, nil, store.InStoreApp)
		mocks.CheckSent(t, nil, adapter.InSendPacket)
		mocks.CheckRecipients(t, nil, adapter.InSendRecipients)
	}

	// -------------------

	{
		testutil.Desc(t, "Send an invalid packet")

		// Build
		an := mocks.NewMockAckNacker()
		adapter := mocks.NewMockAdapter()
		store := newMockStorage()

		// Operate
		broker := New(store, testutil.GetLogger(t, "Broker"))
		err := broker.HandleUp([]byte{1, 2, 3}, an, adapter)

		// Check
		errutil.CheckErrors(t, pointer.String(string(errors.Structural)), err)
		mocks.CheckAcks(t, false, an.InAck)
		CheckRegistrations(t, nil, store.InStoreDevices)
		CheckRegistrations(t, nil, store.InStoreApp)
		mocks.CheckSent(t, nil, adapter.InSendPacket)
		mocks.CheckRecipients(t, nil, adapter.InSendRecipients)
	}

	// -------------------

	{
		testutil.Desc(t, "Send packet, get 2 entries, no valid MIC")

		// Build
		an := mocks.NewMockAckNacker()
		adapter := mocks.NewMockAdapter()
		store := newMockStorage()
		store.OutLookupDevices = []devEntry{
			{
				Recipient: []byte{1, 2, 3},
				AppEUI:    lorawan.EUI64([8]byte{1, 2, 3, 4, 5, 6, 7, 8}),
				DevEUI:    lorawan.EUI64([8]byte{1, 2, 3, 4, 5, 6, 7, 8}),
				NwkSKey:   lorawan.AES128Key([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}),
			},
			{
				Recipient: []byte{1, 2, 3},
				AppEUI:    lorawan.EUI64([8]byte{1, 1, 1, 1, 5, 5, 5, 5}),
				DevEUI:    lorawan.EUI64([8]byte{4, 4, 4, 4, 5, 5, 5, 5}),
				NwkSKey:   lorawan.AES128Key([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 6, 6, 11, 12, 13, 14, 12, 16}),
			},
		}
		data, _ := newBPacket(
			[4]byte{2, 3, 2, 3},
			"Payload",
			[16]byte{1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8},
			5,
		).MarshalBinary()

		// Operate
		broker := New(store, testutil.GetLogger(t, "Broker"))
		err := broker.HandleUp(data, an, adapter)

		// Check
		errutil.CheckErrors(t, pointer.String(string(errors.Behavioural)), err)
		mocks.CheckAcks(t, false, an.InAck)
		CheckRegistrations(t, nil, store.InStoreDevices)
		CheckRegistrations(t, nil, store.InStoreApp)
		mocks.CheckSent(t, nil, adapter.InSendPacket)
		mocks.CheckRecipients(t, nil, adapter.InSendRecipients)
	}

	// -------------------

	{
		testutil.Desc(t, "Send packet, get 2 entries, 1 valid MIC | No downlink")

		// Build
		an := mocks.NewMockAckNacker()
		recipient := mocks.NewMockRecipient()
		adapter := mocks.NewMockAdapter()
		adapter.OutSend = nil
		adapter.OutGetRecipient = recipient
		store := newMockStorage()
		store.OutLookupDevices = []devEntry{
			{
				Recipient: []byte{1, 2, 3},
				AppEUI:    lorawan.EUI64([8]byte{1, 2, 3, 4, 5, 6, 7, 8}),
				DevEUI:    lorawan.EUI64([8]byte{1, 2, 3, 4, 5, 6, 7, 8}),
				NwkSKey:   lorawan.AES128Key([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}),
			},
			{
				Recipient: []byte{1, 2, 3},
				AppEUI:    lorawan.EUI64([8]byte{1, 1, 1, 1, 5, 5, 5, 5}),
				DevEUI:    lorawan.EUI64([8]byte{4, 4, 4, 4, 2, 3, 2, 3}),
				NwkSKey:   lorawan.AES128Key([16]byte{1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8}),
			},
		}
		bpacket := newBPacket(
			[4]byte{2, 3, 2, 3},
			"Payload",
			[16]byte{1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8},
			5,
		)
		data, _ := bpacket.MarshalBinary()
		hpacket, _ := core.NewHPacket(
			store.OutLookupDevices[1].AppEUI,
			store.OutLookupDevices[1].DevEUI,
			bpacket.Payload(),
			bpacket.Metadata(),
		)

		// Operate
		broker := New(store, testutil.GetLogger(t, "Broker"))
		err := broker.HandleUp(data, an, adapter)

		// Check
		errutil.CheckErrors(t, nil, err)
		mocks.CheckAcks(t, true, an.InAck)
		CheckRegistrations(t, nil, store.InStoreDevices)
		CheckRegistrations(t, nil, store.InStoreApp)
		mocks.CheckSent(t, hpacket, adapter.InSendPacket)
		mocks.CheckRecipients(t, []core.Recipient{recipient}, adapter.InSendRecipients)
	}

	// -------------------

	{
		testutil.Desc(t, "Send packet, get 2 entries, 1 valid MIC | Fails to get recipient")

		// Build
		an := mocks.NewMockAckNacker()
		adapter := mocks.NewMockAdapter()
		adapter.OutSend = nil
		adapter.Failures["GetRecipient"] = errors.New(errors.Structural, "Mock Error: Unable to get recipient")
		store := newMockStorage()
		store.OutLookupDevices = []devEntry{
			{
				Recipient: []byte{1, 2, 3},
				AppEUI:    lorawan.EUI64([8]byte{1, 2, 3, 4, 5, 6, 7, 8}),
				DevEUI:    lorawan.EUI64([8]byte{1, 2, 3, 4, 5, 6, 7, 8}),
				NwkSKey:   lorawan.AES128Key([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}),
			},
			{
				Recipient: []byte{1, 2, 3},
				AppEUI:    lorawan.EUI64([8]byte{1, 1, 1, 1, 5, 5, 5, 5}),
				DevEUI:    lorawan.EUI64([8]byte{4, 4, 4, 4, 2, 3, 2, 3}),
				NwkSKey:   lorawan.AES128Key([16]byte{1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8}),
			},
		}
		bpacket := newBPacket(
			[4]byte{2, 3, 2, 3},
			"Payload",
			[16]byte{1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8},
			5,
		)
		data, _ := bpacket.MarshalBinary()

		// Operate
		broker := New(store, testutil.GetLogger(t, "Broker"))
		err := broker.HandleUp(data, an, adapter)

		// Check
		errutil.CheckErrors(t, pointer.String(string(errors.Structural)), err)
		mocks.CheckAcks(t, false, an.InAck)
		CheckRegistrations(t, nil, store.InStoreDevices)
		CheckRegistrations(t, nil, store.InStoreApp)
		mocks.CheckSent(t, nil, adapter.InSendPacket)
		mocks.CheckRecipients(t, nil, adapter.InSendRecipients)
	}

	// -------------------

	{
		testutil.Desc(t, "Send packet, get 2 entries, 1 valid MIC | Fails to send")

		// Build
		an := mocks.NewMockAckNacker()
		recipient := mocks.NewMockRecipient()
		adapter := mocks.NewMockAdapter()
		adapter.OutGetRecipient = recipient
		adapter.Failures["Send"] = errors.New(errors.Operational, "Mock Error: Unable to send")
		store := newMockStorage()
		store.OutLookupDevices = []devEntry{
			{
				Recipient: []byte{1, 2, 3},
				AppEUI:    lorawan.EUI64([8]byte{1, 2, 3, 4, 5, 6, 7, 8}),
				DevEUI:    lorawan.EUI64([8]byte{1, 2, 3, 4, 5, 6, 7, 8}),
				NwkSKey:   lorawan.AES128Key([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}),
			},
			{
				Recipient: []byte{1, 2, 3},
				AppEUI:    lorawan.EUI64([8]byte{1, 1, 1, 1, 5, 5, 5, 5}),
				DevEUI:    lorawan.EUI64([8]byte{4, 4, 4, 4, 2, 3, 2, 3}),
				NwkSKey:   lorawan.AES128Key([16]byte{1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8}),
			},
		}
		bpacket := newBPacket(
			[4]byte{2, 3, 2, 3},
			"Payload",
			[16]byte{1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8},
			5,
		)
		data, _ := bpacket.MarshalBinary()
		hpacket, _ := core.NewHPacket(
			store.OutLookupDevices[1].AppEUI,
			store.OutLookupDevices[1].DevEUI,
			bpacket.Payload(),
			bpacket.Metadata(),
		)

		// Operate
		broker := New(store, testutil.GetLogger(t, "Broker"))
		err := broker.HandleUp(data, an, adapter)

		// Check
		errutil.CheckErrors(t, pointer.String(string(errors.Operational)), err)
		mocks.CheckAcks(t, false, an.InAck)
		CheckRegistrations(t, nil, store.InStoreDevices)
		CheckRegistrations(t, nil, store.InStoreApp)
		mocks.CheckSent(t, hpacket, adapter.InSendPacket)
		mocks.CheckRecipients(t, []core.Recipient{recipient}, adapter.InSendRecipients)
	}
}