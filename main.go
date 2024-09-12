package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
)

const (
	STORAGE_DIR         = "/storage"        // ДИРЕКТОРИЯ ДЛЯ ХРАНЕНИЯ ОБЪЕКТОВ
	UPLOAD_PREFIX_LEN   = len("/upload/")   // ДЛИНА ПРЕФИКСА ДЛЯ МАРШРУТА ЗАГРУЗКИ
	DOWNLOAD_PREFIX_LEN = len("/download/") // ДЛИНА ПРЕФИКСА ДЛЯ МАРШРУТА ЗАГРУЗКИ
)

// Storage — структура для хранения объектов в памяти
type Storage struct {
	mu    sync.Mutex        // Мьютекс для обеспечения потокобезопасности
	files map[string][]byte // Хэш-таблица для хранения данных объектов
}

// NewStorage — конструктор для создания нового хранилища
func NewStorage() *Storage {
	return &Storage{
		files: make(map[string][]byte),
	}
}

// Save — метод для сохранения объекта в хранилище
func (s *Storage) Save(key string, data []byte) {
	s.mu.Lock()         // Захватываем мьютекс перед записью
	defer s.mu.Unlock() // Освобождаем мьютекс после записи

	// Сохраняем данные в памяти
	s.files[key] = data

	// Также сохраняем данные на диск
	err := ioutil.WriteFile(STORAGE_DIR+"/"+key, data, 0644)
	if err != nil {
		log.Printf("Ошибка при сохранении файла %s: %v", key, err)
	}
}

// Load — метод для загрузки объекта из хранилища
func (s *Storage) Load(key string) ([]byte, bool) {
	s.mu.Lock()         // Захватываем мьютекс перед чтением
	defer s.mu.Unlock() // Освобождаем мьютекс после чтения

	// Проверяем наличие объекта в памяти
	data, exists := s.files[key]
	if exists {
		return data, true
	}

	// Если объект не найден в памяти, пытаемся загрузить его с диска
	data, err := ioutil.ReadFile(STORAGE_DIR + "/" + key)
	if err != nil {
		return nil, false
	}

	// Если загрузка с диска успешна, кэшируем объект в памяти
	s.files[key] = data
	return data, true
}

// HandleUpload — обработчик для загрузки объектов
func HandleUpload(w http.ResponseWriter, r *http.Request, storage *Storage) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	// Получаем ключ (имя объекта) из URL
	key := r.URL.Path[UPLOAD_PREFIX_LEN:]

	// Читаем тело запроса (данные объекта)
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Ошибка чтения данных", http.StatusInternalServerError)
		return
	}

	// Сохраняем объект в хранилище
	storage.Save(key, data)

	// Отправляем ответ клиенту
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Объект %s успешно сохранен", key)
}

// HandleDownload — обработчик для загрузки объектов
func HandleDownload(w http.ResponseWriter, r *http.Request, storage *Storage) {
	if r.Method != http.MethodGet {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	// Получаем ключ (имя объекта) из URL
	key := r.URL.Path[DOWNLOAD_PREFIX_LEN:]

	// Загружаем объект из хранилища
	data, exists := storage.Load(key)
	if !exists {
		http.Error(w, "Объект не найден", http.StatusNotFound)
		return
	}

	// Отправляем данные объекта клиенту
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// HandleList — обработчик для вывода списка всех объектов
func HandleList(w http.ResponseWriter, r *http.Request, storage *Storage) {
	if r.Method != http.MethodGet {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	// Захватываем мьютекс для доступа к хэш-таблице объектов
	storage.mu.Lock()
	defer storage.mu.Unlock()

	// Создаем список ключей (имен объектов)
	keys := make([]string, 0, len(storage.files))
	for key := range storage.files {
		keys = append(keys, key)
	}

	// Кодируем список ключей в формат JSON и отправляем клиенту
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(keys)
}

func main() {
	// Проверяем наличие директории для хранения объектов
	if _, err := os.Stat(STORAGE_DIR); os.IsNotExist(err) {
		err := os.Mkdir(STORAGE_DIR, 0755)
		if err != nil {
			log.Fatalf("Ошибка создания директории %s: %v", STORAGE_DIR, err)
		}
	}

	// Создаем новое хранилище
	storage := NewStorage()

	// Настраиваем маршруты для обработки HTTP-запросов
	http.HandleFunc("/upload/", func(w http.ResponseWriter, r *http.Request) {
		HandleUpload(w, r, storage)
	})
	http.HandleFunc("/download/", func(w http.ResponseWriter, r *http.Request) {
		HandleDownload(w, r, storage)
	})
	http.HandleFunc("/list", func(w http.ResponseWriter, r *http.Request) {
		HandleList(w, r, storage)
	})

	// Запускаем HTTP-сервер на порту 8080
	log.Println("Сервер запущен на порту 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
