package services

import (
	"errors"
	"feed/cache"
	"feed/config"
	"feed/models"
	"feed/repository"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"golang.org/x/crypto/bcrypt"
)

type registrationRepo struct {
	repository.UserRepository
	users         []*models.User
	usernameTaken bool
	queryError    error
}

func (r *registrationRepo) CountByUsername(string) (int64, error) {
	if r.usernameTaken {
		return 1, nil
	}
	return 0, r.queryError
}
func (r *registrationRepo) CountByEmail(string) (int64, error) { return 0, r.queryError }
func (r *registrationRepo) Create(user *models.User) error {
	user.ID = 1
	r.users = append(r.users, user)
	return nil
}

func registrationSetup(t *testing.T) (*miniredis.Miniredis, *UserService, *registrationRepo) {
	t.Helper()
	server := miniredis.RunT(t)
	oldClient := cache.RedisClient
	oldConfig := config.AppConfig
	config.AppConfig = &config.Config{}
	cache.RedisClient = redis.NewClient(&redis.Options{Addr: server.Addr()})
	config.AppConfig.Email.CodeMaxAttempts = 3
	config.AppConfig.Email.PendingTTLMin = 1
	t.Cleanup(func() { cache.RedisClient.Close(); cache.RedisClient = oldClient; config.AppConfig = oldConfig })
	repo := &registrationRepo{}
	return server, &UserService{userRepo: repo}, repo
}

func registrationTicket(t *testing.T, service *UserService) string {
	t.Helper()
	code, err := cache.SaveVerificationCode(EmailSceneRegister, "test@example.com", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	token, err := service.RegisterVerify(&RegisterConfirmRequest{Email: " Test@Example.com ", Code: code})
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func TestRegistrationEmailFirst(t *testing.T) {
	_, service, repo := registrationSetup(t)
	token := registrationTicket(t, service)
	if len(repo.users) != 0 {
		t.Fatal("verification created an account")
	}
	req := &RegisterRequest{Email: "test@example.com", Username: "tester", Nickname: "Test", Password: "password123", RegistrationToken: token}
	repo.usernameTaken = true
	if _, err := service.Register(req); err == nil {
		t.Fatal("accepted occupied username")
	}
	if err := cache.CheckRegistrationTicket(token, req.Email); err != nil {
		t.Fatal("validation consumed ticket", err)
	}
	repo.usernameTaken = false
	repo.queryError = errors.New("database unavailable")
	if _, err := service.Register(req); err == nil {
		t.Fatal("ignored query failure")
	}
	repo.queryError = nil
	user, err := service.Register(req)
	if err != nil {
		t.Fatal(err)
	}
	if !user.EmailVerified || *user.Email != req.Email {
		t.Fatal("email was not bound")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Register(req); !errors.Is(err, cache.ErrRegistrationExpired) {
		t.Fatal("replay accepted", err)
	}
	if len(repo.users) != 1 {
		t.Fatal("unexpected account count")
	}
}

func TestRegistrationTicketBindingAndExpiry(t *testing.T) {
	server, service, repo := registrationSetup(t)
	token := registrationTicket(t, service)
	for _, req := range []*RegisterRequest{
		{Email: "test@example.com"},
		{Email: "other@example.com", RegistrationToken: token},
	} {
		if _, err := service.Register(req); !errors.Is(err, cache.ErrRegistrationExpired) {
			t.Fatal("invalid ticket accepted", err)
		}
	}
	if err := cache.CheckRegistrationTicket(token, "test@example.com"); err != nil {
		t.Fatal(err)
	}
	server.FastForward(2 * time.Minute)
	if err := cache.CheckRegistrationTicket(token, "test@example.com"); !errors.Is(err, cache.ErrRegistrationExpired) {
		t.Fatal("expired ticket accepted")
	}
	if len(repo.users) != 0 {
		t.Fatal("invalid request created account")
	}
}

func TestRegistrationConcurrentExchangeAndConsume(t *testing.T) {
	_, service, _ := registrationSetup(t)
	code, err := cache.SaveVerificationCode(EmailSceneRegister, "test@example.com", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	tokens := make(chan string, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			token, err := service.RegisterVerify(&RegisterConfirmRequest{Email: "test@example.com", Code: code})
			if err == nil {
				tokens <- token
			}
		}()
	}
	wg.Wait()
	close(tokens)
	if len(tokens) != 1 {
		t.Fatalf("issued %d tickets from one code", len(tokens))
	}
	token := <-tokens
	var successes atomic.Int32
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if cache.ConsumeRegistrationTicket(token, "test@example.com") == nil {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatal("ticket consumed more than once")
	}
}

func TestRegistrationCodeAttemptsAndScene(t *testing.T) {
	_, service, _ := registrationSetup(t)
	code, err := cache.SaveVerificationCode(EmailSceneForgotPassword, "test@example.com", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.RegisterVerify(&RegisterConfirmRequest{Email: "test@example.com", Code: code}); !errors.Is(err, cache.ErrCodeExpired) {
		t.Fatal("accepted another scene's code")
	}
	code, err = cache.SaveVerificationCode(EmailSceneRegister, "test@example.com", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		_, err = service.RegisterVerify(&RegisterConfirmRequest{Email: "test@example.com", Code: "wrong"})
		if err == nil {
			t.Fatal("accepted wrong code")
		}
	}
	if !errors.Is(err, cache.ErrCodeTooManyAttempts) {
		t.Fatal(err)
	}
	if _, err := service.RegisterVerify(&RegisterConfirmRequest{Email: "test@example.com", Code: code}); !errors.Is(err, cache.ErrCodeExpired) {
		t.Fatal("accepted exhausted code")
	}
}
