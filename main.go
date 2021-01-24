package main

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/joaomarcelofa/entendendo-worker-pool/urls"
)

// Result é uma estrutura de dados que representa um par de URL x Tempo de reposta
type Result struct {
	URL        string
	TimeTooked time.Duration
}

func main() {
	fmt.Println("Method 1 - Sequential")
	start := time.Now()
	result := getFastestURLSequential(urls.List)
	elapsed := time.Since(start)
	fmt.Printf("Fastest URL: %s - %s\n", result.URL, result.TimeTooked)
	fmt.Printf("Total time tooked on Method 1: %s\n", elapsed)

	fmt.Printf("\n\n\n")

	fmt.Println("Method 2 - Worker pool")
	start = time.Now()
	result = getFastestURLWorkerPool(urls.List)
	elapsed = time.Since(start)
	fmt.Printf("Fastest URL: %s - %s\n", result.URL, result.TimeTooked)
	fmt.Printf("Total time tooked on Method 2: %s\n", elapsed)
}

func createSimpleHTTPClient(timeout int) *http.Client {
	// Cria um cliente http
	return &http.Client{
		Timeout: time.Second * time.Duration(timeout),
	}
}

func visitURL(client *http.Client, url string) (time.Duration, error) {
	// Monta a requisição
	req, err := http.NewRequest("GET", url, nil)
	// Começa a contar o tempo
	start := time.Now()
	// Efetua a requisição
	resp, err := client.Do(req)
	if err != nil {
		return time.Duration(0), err
	}
	// Finaliza a contagem do tempo
	elapsed := time.Since(start)
	// Verifica se a requisição teve sucesso de acordo com o código retornado
	if resp.StatusCode != 200 {
		return time.Duration(0), errors.New("Status code 200 not returned")
	}
	return elapsed, nil
}

func getFastestURLSequential(urls []string) Result {
	// Declarando a variável que irá armazenar a URL com o tempo de resposta mais rápida e
	// o próprio tempo de resposta
	var fastestTime time.Duration
	fastestURL := ""

	httpClient := createSimpleHTTPClient(5)

	// Visitando todas as URLs da lista de URLs
	for _, url := range urls {
		// Visitando a URL medindo o tempo de resposta
		elapsed, err := visitURL(httpClient, url)
		// Verificando se houve erro com a requisição
		if err != nil {
			// Em caso de erro, o tempo de solicitação será desconsiderado
			fmt.Printf("Error at getting url %s\nError: %s\n", url, err.Error())
			continue
		}
		fmt.Printf("Visited %s - Took: %s\n", url, elapsed)

		// Atualizando o menor tempo
		if fastestTime == time.Duration(0) {
			// Na primeira iteração, o tempo mais rápido, será 0, então a primeira resposta é automaticamente a mais rápida
			fastestTime = elapsed
			fastestURL = url
		} else if elapsed < fastestTime {
			// Caso o tempo da requisição atual seja menor que o menor tempo, o tempo mais rápido é atualizado juntamente
			// com a url que resultou neste tempo
			fastestTime = elapsed
			fastestURL = url
		}
	}

	return Result{
		URL:        fastestURL,
		TimeTooked: fastestTime,
	}
}

func getFastestURLWorkerPool(urls []string) Result {
	// 1. Declarando um waiting group para sincronizar todos os workers
	// Obs: O grupo de espera deve ter o mesmo tamanho da lista de URLs recebidas
	var wg sync.WaitGroup
	wg.Add(len(urls))

	// 2. Declarando a variável compatilhada para armazenar o resultado da URL mais rápida
	// Apesar desta variável ser compartilhada, sua declaração não difere das outras, pois
	// sua referência será enviada para o worker
	var fastestResult Result
	// 3. Declarando a variável de exclusão mútua para garantir a atualização correta da variável
	// fastestResult
	var mux sync.Mutex

	// 4. Declarando os workers
	qtyWorkers := 8 // Altere o número de workers aqui
	urlCh := make(chan string, qtyWorkers)

	// 5. Criando os workers
	for i := 0; i < qtyWorkers; i++ {
		// Criando uma goroutine para cada worker
		go getFastestURLByWorker(urlCh, &wg, &mux, &fastestResult)
	}

	// 6. Distribuindo as URLs para os workers através do channel
	for _, url := range urls {
		urlCh <- url
	}

	// 7. Ponto de espera até que o waiting group tenha sua condição satisfeita, ou seja,
	// esperar por todas as requisições retornarem
	wg.Wait()

	return fastestResult
}

// A função getFastestURLByWorker deve receber o canal de Urls, assim como as referências do grupo de espera,
// da variável de controle de acesso à variável compartilhada e a referência da variável compartilhada
func getFastestURLByWorker(urlCh <-chan string, wg *sync.WaitGroup, mux *sync.Mutex, fastestResult *Result) {
	httpClient := createSimpleHTTPClient(5)
	// Visitando a URL recebida pelo channel
	for url := range urlCh {
		// Visitando a URL medindo o tempo de resposta
		elapsed, err := visitURL(httpClient, url)
		// Verificando se houve erro com a requisição
		if err != nil {
			fmt.Printf("Error at getting url %s\nError: %s\n", url, err.Error())
		} else {
			fmt.Printf("Visited %s - Took: %s\n", url, elapsed)
			// Restringindo o acesso simultâneo a variável compartilhada
			mux.Lock()

			// Atualizando o menor tempo
			if fastestResult.TimeTooked == time.Duration(0) {
				// Na primeira iteração, o tempo mais rápido, será 0, então a primeira resposta é automaticamente a mais rápida
				fastestResult.TimeTooked = elapsed
				fastestResult.URL = url
			} else if elapsed < fastestResult.TimeTooked {
				// Caso o tempo da requisição atual seja menor que o menor tempo, o tempo mais rápido é atualizado juntamente
				// com a url que resultou neste tempo
				fastestResult.TimeTooked = elapsed
				fastestResult.URL = url
			}
			// Liberando o acesso das outras goroutines a variável compartilhada
			mux.Unlock()
		}
		// Marca que uma URL foi visitada
		wg.Done()
	}
}
