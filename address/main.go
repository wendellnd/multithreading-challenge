package address

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

const DEFAULT_TIMEOUT = 30 * time.Second

type AddressResult struct {
	Source       string
	State        string
	City         string
	Street       string
	ZipCode      string
	Neighborhood string
}

type GetAddressFunc func(ctx context.Context, client http.Client, wg *sync.WaitGroup, ch chan AddressResult, cancel context.CancelFunc, cep string)

type AddressService struct {
	Timeout   time.Duration
	client    http.Client
	ctx       context.Context
	cancel    context.CancelFunc
	functions []GetAddressFunc
}

func NewAddressService(ctx context.Context) *AddressService {
	client := http.Client{
		Timeout: DEFAULT_TIMEOUT,
	}
	ctx, cancel := context.WithCancel(ctx)

	return &AddressService{
		client: client,
		ctx:    ctx,
		cancel: cancel,
		functions: []GetAddressFunc{
			ViaCEP,
			BrasilAPI,
		},
	}
}

func (s *AddressService) SetTimeout(timeout time.Duration) *AddressService {
	s.Timeout = timeout
	s.client.Timeout = timeout
	return s
}

func (s *AddressService) Execute(cep string) (address AddressResult, err error) {
	defer s.cancel()

	length := len(s.functions)
	ch := make(chan AddressResult, length)
	var wg sync.WaitGroup

	for _, f := range s.functions {
		wg.Add(1)
		go f(s.ctx, s.client, &wg, ch, s.cancel, cep)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	select {
	case <-time.After(s.Timeout):
		message := "request timeout"
		return address, errors.New(message)
	case <-s.ctx.Done():
		return <-ch, nil
	}
}

type BrasilAPIResponse struct {
	CEP          string `json:"cep"`
	City         string `json:"city"`
	Neighborhood string `json:"neighborhood"`
	State        string `json:"state"`
	Street       string `json:"street"`
}

func (r BrasilAPIResponse) ToAddressResult() AddressResult {
	return AddressResult{
		Source:       "BrasilAPI",
		State:        r.State,
		City:         r.City,
		Street:       r.Street,
		ZipCode:      r.CEP,
		Neighborhood: r.Neighborhood,
	}
}

func BrasilAPI(ctx context.Context, client http.Client, wg *sync.WaitGroup, ch chan AddressResult, cancel context.CancelFunc, cep string) {
	defer wg.Done()

	result := AddressResult{
		Source: "BrasilAPI",
	}

	url := fmt.Sprintf("https://brasilapi.com.br/api/cep/v1/%s", cep)

	request, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Println(err)
		return
	}

	response, err := client.Do(request)
	if err != nil {
		if os.IsTimeout(err) {
			log.Println("Timeout, source: ", result.Source)
			return
		}

		log.Println(err)
		return
	}
	defer response.Body.Close()

	var brasilAPIResponse BrasilAPIResponse
	err = json.NewDecoder(response.Body).Decode(&brasilAPIResponse)
	if err != nil {
		log.Println(err)
		return
	}

	ch <- brasilAPIResponse.ToAddressResult()
	cancel()
}

type ViaCEPResponse struct {
	CEP          string `json:"cep"`
	City         string `json:"localidade"`
	Neighborhood string `json:"bairro"`
	State        string `json:"uf"`
	Street       string `json:"logradouro"`
}

func (r ViaCEPResponse) ToAddressResult() AddressResult {
	return AddressResult{
		Source:       "ViaCEP",
		State:        r.State,
		City:         r.City,
		Street:       r.Street,
		ZipCode:      r.CEP,
		Neighborhood: r.Neighborhood,
	}
}

func ViaCEP(ctx context.Context, client http.Client, wg *sync.WaitGroup, ch chan AddressResult, cancel context.CancelFunc, cep string) {
	defer wg.Done()

	result := AddressResult{
		Source: "ViaCEP",
	}
	url := fmt.Sprintf("http://viacep.com.br/ws/%s/json", cep)

	request, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Println(err)
		return
	}

	response, err := client.Do(request)
	if err != nil {
		if os.IsTimeout(err) {
			log.Println("Timeout, source: ", result.Source)
			return
		}

		log.Println(err)
		return
	}
	defer response.Body.Close()

	var viaCepResponse ViaCEPResponse
	err = json.NewDecoder(response.Body).Decode(&viaCepResponse)
	if err != nil {
		log.Println(err)
		return
	}

	ch <- viaCepResponse.ToAddressResult()
	cancel()
}
