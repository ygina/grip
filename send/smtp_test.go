package send

import (
	"net/mail"
	"strings"
	"testing"

	"github.com/mongodb/grip/level"
	"github.com/mongodb/grip/message"
	"github.com/stretchr/testify/suite"
)

type SMTPSuite struct {
	opts *SMTPOptions
	suite.Suite
}

func TestSMTPSuite(t *testing.T) {
	suite.Run(t, new(SMTPSuite))
}

func (s *SMTPSuite) SetupSuite() {}

func (s *SMTPSuite) SetupTest() {
	s.opts = &SMTPOptions{
		client:        &smtpClientMock{},
		Subject:       "test email from logger",
		NameAsSubject: true,
		Name:          "test smtp sender",
		toAddrs: []*mail.Address{
			{
				Name:    "one",
				Address: "two",
			},
		},
	}
	s.Nil(s.opts.GetContents)
	s.NoError(s.opts.Validate())
	s.NotNil(s.opts.GetContents)
}

func (s *SMTPSuite) TestOptionsMustBeIValid() {
	invalidOpts := []*SMTPOptions{
		{},
		{
			Subject:          "too many subject uses",
			NameAsSubject:    true,
			MessageAsSubject: true,
		},
		{
			Subject: "missing name",
			toAddrs: []*mail.Address{
				{
					Name:    "one",
					Address: "two",
				},
			},
		},
		{
			Subject: "empty",
			Name:    "sender",
			toAddrs: []*mail.Address{},
		},
	}

	for _, opts := range invalidOpts {
		s.Error(opts.Validate())
	}
}

func (s *SMTPSuite) TestDefaultGetContents() {
	s.NotNil(s.opts)

	m := message.NewString("helllooooo!")
	sbj, msg := s.opts.GetContents(s.opts, m)

	s.True(s.opts.NameAsSubject)
	s.Equal(s.opts.Name, sbj)
	s.Equal(m.String(), msg)

	s.opts.NameAsSubject = false
	sbj, _ = s.opts.GetContents(s.opts, m)
	s.Equal(s.opts.Subject, sbj)

	s.opts.MessageAsSubject = true
	sbj, msg = s.opts.GetContents(s.opts, m)
	s.Equal("", msg)
	s.Equal(m.String(), sbj)
	s.opts.MessageAsSubject = false

	s.opts.Subject = ""
	sbj, msg = s.opts.GetContents(s.opts, m)
	s.Equal("", sbj)
	s.Equal(m.String(), msg)
	s.opts.Subject = "test email subject"

	s.opts.TruncatedMessageSubjectLength = len(m.String()) * 2
	sbj, msg = s.opts.GetContents(s.opts, m)
	s.Equal(m.String(), msg)
	s.Equal(m.String(), sbj)

	s.opts.TruncatedMessageSubjectLength = len(m.String()) - 2
	sbj, msg = s.opts.GetContents(s.opts, m)
	s.Equal(m.String(), msg)
	s.NotEqual(msg, sbj)
	s.True(len(msg) > len(sbj))
}

func (s *SMTPSuite) TestResetRecips() {
	s.True(len(s.opts.toAddrs) > 0)
	s.opts.ResetRecipients()
	s.Len(s.opts.toAddrs, 0)
}

func (s *SMTPSuite) TestAddRecipientsFailsWithNoArgs() {
	s.opts.ResetRecipients()
	s.Error(s.opts.AddRecipients())
	s.Len(s.opts.toAddrs, 0)
}

func (s *SMTPSuite) TestAddRecipientsErrorsWithInvalidAddresses() {
	s.opts.ResetRecipients()
	s.Error(s.opts.AddRecipients("foo", "bar", "baz"))
	s.Len(s.opts.toAddrs, 0)
}

func (s *SMTPSuite) TestAddingMultipleRecipients() {
	s.opts.ResetRecipients()

	s.NoError(s.opts.AddRecipients("test <one@example.net>"))
	s.Len(s.opts.toAddrs, 1)
	s.NoError(s.opts.AddRecipients("test <one@example.net>", "test2 <two@example.net>"))
	s.Len(s.opts.toAddrs, 3)
}

func (s *SMTPSuite) TestAddingSingleRecipientWithInvalidAddressErrors() {
	s.opts.ResetRecipients()
	s.Error(s.opts.AddRecipient("test", "address"))
	s.Len(s.opts.toAddrs, 0)
	s.Error(s.opts.AddRecipient("test <one@example.net>", "test2 <two@example.net>"))
	s.Len(s.opts.toAddrs, 0)
}

func (s *SMTPSuite) TestAddingSingleRecipient() {
	s.opts.ResetRecipients()
	s.NoError(s.opts.AddRecipient("test", "one@example.net"))
	s.Len(s.opts.toAddrs, 1)
}

func (s *SMTPSuite) TestMakeConstructorFailureCases() {
	sender, err := MakeSMTPLogger(nil)
	s.Nil(sender)
	s.Error(err)

	sender, err = MakeSMTPLogger(&SMTPOptions{})
	s.Nil(sender)
	s.Error(err)

	s.opts.client = &smtpClientMock{
		failCreate: true,
	}

	sender, err = MakeSMTPLogger(s.opts)
	s.Nil(sender)
	s.Error(err)
}

func (s *SMTPSuite) TestDefaultSmtpImplShouldValidate() {
	s.opts.client = nil
	s.NoError(s.opts.Validate())
	s.NotNil(s.opts.client)

	s.Error(s.opts.client.Create(s.opts))
	s.opts.UseSSL = true
	s.Error(s.opts.client.Create(s.opts))
}

func (s *SMTPSuite) TestSendMailErrorsIfNoAddresses() {
	s.opts.ResetRecipients()
	s.Len(s.opts.toAddrs, 0)

	m := message.NewString("hello world!")
	s.Error(s.opts.sendMail(m))
}

func (s *SMTPSuite) TestSendMailErrorsIfMailCallFails() {
	s.opts.client = &smtpClientMock{
		failMail: true,
	}

	m := message.NewString("hello world!")
	s.Error(s.opts.sendMail(m))
}

func (s *SMTPSuite) TestSendMailErrorsIfRecptFails() {
	s.opts.client = &smtpClientMock{
		failRcpt: true,
	}

	m := message.NewString("hello world!")
	s.Error(s.opts.sendMail(m))
}

func (s *SMTPSuite) TestSendMailErrorsIfDataFails() {
	s.opts.client = &smtpClientMock{
		failData: true,
	}

	m := message.NewString("hello world!")
	s.Error(s.opts.sendMail(m))
}

func (s *SMTPSuite) TestSendMailRecordsMessage() {
	m := message.NewString("hello world!")
	s.NoError(s.opts.sendMail(m))
	mock, ok := s.opts.client.(*smtpClientMock)
	s.Require().True(ok)
	s.True(strings.Contains(mock.message.String(), s.opts.Name))
	s.True(strings.Contains(mock.message.String(), "plain"))
	s.False(strings.Contains(mock.message.String(), "html"))

	s.opts.PlainTextContents = false
	s.NoError(s.opts.sendMail(m))
	s.True(strings.Contains(mock.message.String(), s.opts.Name))
	s.True(strings.Contains(mock.message.String(), "html"))
	s.False(strings.Contains(mock.message.String(), "plain"))
}

func (s *SMTPSuite) TestNewConstructor() {
	sender, err := NewSMTPLogger(nil, LevelInfo{level.Trace, level.Info})
	s.Error(err)
	s.Nil(sender)

	sender, err = NewSMTPLogger(s.opts, LevelInfo{level.Invalid, level.Info})
	s.Error(err)
	s.Nil(sender)

	sender, err = NewSMTPLogger(s.opts, LevelInfo{level.Trace, level.Info})
	s.NoError(err)
	s.NotNil(sender)
}

func (s *SMTPSuite) TestSendMethod() {
	sender, err := NewSMTPLogger(s.opts, LevelInfo{level.Trace, level.Info})
	s.NoError(err)
	s.NotNil(sender)

	mock, ok := s.opts.client.(*smtpClientMock)
	s.True(ok)
	s.Equal(mock.numMsgs, 0)

	m := message.NewDefaultMessage(level.Debug, "hello")
	sender.Send(m)
	s.Equal(mock.numMsgs, 0)

	m = message.NewDefaultMessage(level.Alert, "")
	sender.Send(m)
	s.Equal(mock.numMsgs, 0)

	m = message.NewDefaultMessage(level.Alert, "world")
	sender.Send(m)
	s.Equal(mock.numMsgs, 1)
}

func (s *SMTPSuite) TestSendMethodWithError() {
	sender, err := NewSMTPLogger(s.opts, LevelInfo{level.Trace, level.Info})
	s.NoError(err)
	s.NotNil(sender)

	mock, ok := s.opts.client.(*smtpClientMock)
	s.True(ok)
	s.Equal(mock.numMsgs, 0)
	s.False(mock.failData)

	m := message.NewDefaultMessage(level.Alert, "world")
	sender.Send(m)
	s.Equal(mock.numMsgs, 1)

	mock.failData = true
	sender.Send(m)
	s.Equal(mock.numMsgs, 1)
}
