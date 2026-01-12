package services

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"gorm.io/gorm"
)

const emailExportMaxEMLBytes = 15 * 1024 * 1024

func (s *EmailService) ExportEmailLogEMLCtx(ctx context.Context, ownerUserID string, logID string) (filename string, emlBytes []byte, err error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return "", nil, fmt.Errorf("missing owner_user_id")
	}
	logID = strings.TrimSpace(logID)
	if logID == "" {
		return "", nil, fmt.Errorf("missing log id")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	logRow, err := s.repo.FindLogByIDCtx(ctx, logID)
	if err != nil {
		return "", nil, err
	}
	if strings.TrimSpace(logRow.OwnerUserID) != ownerUserID {
		return "", nil, gorm.ErrRecordNotFound
	}
	if logRow.MessageUID <= 0 {
		return "", nil, fmt.Errorf("missing message uid (old log record); please re-fetch emails")
	}

	cfg, err := s.repo.FindConfigByIDCtx(ctx, logRow.EmailConfigID)
	if err != nil {
		return "", nil, err
	}
	if strings.TrimSpace(cfg.OwnerUserID) != ownerUserID {
		return "", nil, fmt.Errorf("email config owner mismatch")
	}

	addr := fmt.Sprintf("%s:%d", cfg.IMAPHost, cfg.IMAPPort)
	// #nosec G402 - InsecureSkipVerify is intentional to support self-signed certs
	c, err := client.DialTLS(addr, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		return "", nil, err
	}
	defer c.Logout()

	if err := c.Login(cfg.Email, cfg.Password); err != nil {
		return "", nil, err
	}

	mailbox := strings.TrimSpace(logRow.Mailbox)
	if mailbox == "" {
		mailbox = "INBOX"
	}
	if _, err := c.Select(mailbox, false); err != nil {
		return "", nil, err
	}

	seqSet := new(imap.SeqSet)
	seqSet.AddNum(logRow.MessageUID)
	section := &imap.BodySectionName{Peek: true} // BODY.PEEK[]
	items := []imap.FetchItem{imap.FetchUid, section.FetchItem()}

	msgCh := make(chan *imap.Message, 1)
	fetchErr := make(chan error, 1)
	go func() {
		fetchErr <- c.UidFetch(seqSet, items, msgCh)
	}()

	var msg *imap.Message
	select {
	case <-ctx.Done():
		return "", nil, ctx.Err()
	case m := <-msgCh:
		msg = m
	case err := <-fetchErr:
		if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			return "", nil, err
		}
	}

	if msg == nil {
		// Give the fetch a short chance to deliver the message if it raced with the error channel.
		select {
		case <-ctx.Done():
			return "", nil, ctx.Err()
		case m := <-msgCh:
			msg = m
		case <-time.After(300 * time.Millisecond):
		}
	}

	if msg == nil {
		return "", nil, gorm.ErrRecordNotFound
	}

	r := msg.GetBody(section)
	if r == nil {
		return "", nil, fmt.Errorf("message body not available")
	}

	b, err := readWithLimit(r, emailExportMaxEMLBytes)
	if err != nil {
		return "", nil, err
	}

	filename = fmt.Sprintf("email_%d.eml", logRow.MessageUID)
	return filename, b, nil
}

