package cdek

// service_types.go - Удобные типы для высокоуровневого Service API (PascalCase по регламенту)
//
// Эти типы являются обертками над автогенерированными CDEK типами,
// упрощающими использование библиотеки в приложениях.

// ========================
// Cost Calculation
// ========================

// CostRequest - запрос на расчет стоимости доставки
type CostRequest struct {
	FromCityCode int32     // Код города отправителя (по КЛАДР)
	ToCityCode   int32     // Код города получателя (по КЛАДР)
	Packages     []Package // Список мест (упаковок)
}

// Package - информация об одной упаковке (месте)
type Package struct {
	Weight int32 // Вес в граммах
	Length int32 // Длина в см
	Width  int32 // Ширина в см
	Height int32 // Высота в см
}

// CostResponse - ответ с вариантами доставки и стоимостью
type CostResponse struct {
	Tariffs []TariffOption // Доступные тарифы
}

// TariffOption - один вариант тарифа доставки
type TariffOption struct {
	TariffCode   int     // Код тарифа
	TariffName   string  // Название тарифа
	DeliveryMode int     // Режим доставки: 1=дверь-дверь, 2=дверь-склад, 3=склад-дверь, 4=склад-склад
	DeliverySum  float64 // Стоимость доставки (рубли)
	PeriodMin    int     // Минимальный срок доставки (дней)
	PeriodMax    int     // Максимальный срок доставки (дней)
}

// ========================
// Orders
// ========================

// Seller - истинный продавец (третье лицо)
// Используется для интернет-магазинов, когда фактический продавец отличается от отправителя
type Seller struct {
	Name          *string // Наименование истинного продавца
	INN           *string // ИНН истинного продавца (10 или 12 символов)
	Phone         *string // Телефон истинного продавца
	OwnershipForm *int    // Код формы собственности
	Address       *string // Адрес истинного продавца
}

// OrderRequest - запрос на создание заказа
type OrderRequest struct {
	Type         string        // Тип заказа: "delivery" (доставка), "pickup" (самовывоз)
	TariffCode   int           // Код тарифа
	Comment      *string       // Комментарий к заказу
	Sender       Recipient     // Отправитель (может быть компания с ИНН или физлицо с паспортом)
	Recipient    Recipient     // Получатель (может быть компания с ИНН или физлицо с паспортом)
	Seller       *Seller       // Истинный продавец (третье лицо) - только для интернет-магазинов
	FromLocation Location      // Адрес отправителя
	ToLocation   Location      // Адрес получателя
	Packages     []OrderPackage // Список мест
}

// Contact - контактная информация
type Contact struct {
	Company *string // Название компании
	Name    string  // ФИО контактного лица
	Email   *string // Email
	Phones  []Phone // Список телефонов
}

// Recipient - информация о получателе (расширяет Contact)
// Может быть как физическое лицо (Name + паспорт), так и компания (Company + ИНН)
type Recipient struct {
	Contact                 // Базовая контактная информация (Name обязательно для физлица, Company для юрлица)
	TIN                     *string // ИНН (Tax Identification Number) - для юридических лиц и ИП (10 или 12 символов)
	PassportSeries          *string // Серия паспорта (для физических лиц)
	PassportNumber          *string // Номер паспорта (для физических лиц)
	PassportDateOfIssue     *string // Дата выдачи паспорта (для физических лиц)
	PassportOrganization    *string // Кем выдан паспорт (для физических лиц)
	PassportDateOfBirth     *string // Дата рождения (yyyy-MM-dd) (для физических лиц)
}

// Phone - телефонный номер
type Phone struct {
	Number     string  // Номер телефона (обязательно)
	Additional *string // Добавочный номер
}

// Location - местоположение (адрес)
type Location struct {
	Code       *int32  // Код населенного пункта СДЭК
	FiasGUID   *string // Уникальный идентификатор ФИАС
	PostalCode *string // Почтовый индекс
	CountryCode *string // Код страны (ISO 3166-1 alpha-2)
	Region     *string // Регион
	City       *string // Город
	Address    *string // Адрес (улица, дом, квартира)
}

// OrderPackage - информация об упаковке в заказе
type OrderPackage struct {
	Number  string  // Номер упаковки (артикул)
	Weight  int32   // Общий вес (граммы)
	Length  *int32  // Длина (см)
	Width   *int32  // Ширина (см)
	Height  *int32  // Высота (см)
	Comment *string // Комментарий
	Items   []Item  // Список вложений
}

// Item - вложение в упаковку
type Item struct {
	Name    string  // Наименование товара
	WareKey string  // Артикул товара
	Payment float64 // Оплата (за единицу товара, в т.ч. частичная предоплата)
	Cost    float64 // Объявленная стоимость товара (за единицу)
	Weight  int32   // Вес (граммы, за единицу)
	Amount  int32   // Количество единиц товара
}

// OrderResponse - ответ при создании заказа
type OrderResponse struct {
	UUID       string        // Идентификатор заказа в CDEK
	Number     *string       // Номер заказа CDEK (может быть null до обработки)
	TariffCode int           // Код тарифа
	Statuses   []StatusEvent // История статусов
	CreatedAt  string        // Дата и время создания (ISO 8601)
}

// StatusEvent - событие изменения статуса
type StatusEvent struct {
	Code     string  // Код статуса
	Name     string  // Название статуса
	DateTime string  // Дата и время статуса (ISO 8601)
	City     *string // Город, в котором произошло событие
}

// UpdateOrderRequest - запрос на обновление заказа
type UpdateOrderRequest struct {
	OrderUUID    string         // UUID заказа для обновления
	Recipient    *Recipient     // Новые данные получателя
	Sender       *Recipient     // Новые данные отправителя (может быть компания с ИНН или физлицо)
	Seller       *Seller        // Истинный продавец (третье лицо)
	ToLocation   *Location      // Новый адрес доставки
	FromLocation *Location      // Новый адрес отправления
	Comment      *string        // Новый комментарий
	Packages     []OrderPackage // Обновленный список мест (если нужно)
}

// OrderInfo - полная информация о заказе
type OrderInfo struct {
	UUID              string         // Идентификатор заказа в CDEK
	Number            *string        // Номер заказа CDEK
	Type              string         // Тип заказа
	TariffCode        int            // Код тарифа
	Sender            Recipient      // Отправитель (может быть компания с ИНН)
	Recipient         Recipient      // Получатель (может быть компания с ИНН)
	Seller            *Seller        // Продавец (третье лицо, для интернет-магазинов)
	FromLocation      Location       // Адрес отправления
	ToLocation        Location       // Адрес доставки
	Packages          []OrderPackage // Список мест
	Statuses          []StatusEvent  // История статусов
	CreatedAt         string         // Дата создания
	DeliveryCost      *float64       // Стоимость доставки
	EstimatedDelivery *string        // Планируемая дата доставки
	ActualDelivery    *string        // Фактическая дата доставки
}

// ========================
// Tracking
// ========================

// TrackingInfo - информация об отслеживании заказа
type TrackingInfo struct {
	UUID              string        // Идентификатор заказа в CDEK
	Number            *string       // Номер заказа CDEK
	CurrentStatus     StatusEvent   // Текущий статус
	StatusHistory     []StatusEvent // История статусов
	EstimatedDelivery *string       // Планируемая дата доставки (ISO 8601)
	ActualDelivery    *string       // Фактическая дата доставки (ISO 8601)
}

// ========================
// Delivery Points
// ========================

// DeliveryPointsRequest - запрос на получение списка ПВЗ
type DeliveryPointsRequest struct {
	CityCode string // Код города (по КЛАДР)
	Type     string // Тип пункта: "PVZ" (пункт выдачи), "POSTAMAT" (постамат)
}

// DeliveryPoint - пункт выдачи заказов
type DeliveryPoint struct {
	Code        string        // Код ПВЗ
	Name        string        // Название ПВЗ
	Type        string        // Тип: "PVZ", "POSTAMAT"
	Location    PointLocation // Адрес расположения
	WorkTime    string        // Режим работы
	Phones      []Phone       // Телефоны
	Email       *string       // Email
	Note        *string       // Примечание
	OfficeImage *string       // URL изображения офиса
}

// PointLocation - местоположение пункта выдачи
type PointLocation struct {
	Country    string  // Страна
	Region     string  // Регион
	City       string  // Город
	Address    string  // Адрес
	PostalCode string  // Почтовый индекс
	Latitude   float64 // Широта
	Longitude  float64 // Долгота
}

// ========================
// Print (Barcode/Waybill)
// ========================

// PrintBarcodeRequest - запрос на создание этикеток
type PrintBarcodeRequest struct {
	Orders []PrintOrder // Список заказов для печати
	Copy   *int         // Количество копий (по умолчанию 1)
	Format *string      // Формат: "A4", "A5", "A6" (по умолчанию A4)
}

// PrintWaybillRequest - запрос на создание накладных
type PrintWaybillRequest struct {
	Orders []PrintOrder // Список заказов для печати
	Copy   *int         // Количество копий (по умолчанию 1)
	Format *string      // Формат: "A4", "A5" (по умолчанию A4)
}

// PrintOrder - заказ для печати
type PrintOrder struct {
	OrderUUID string // UUID заказа
}

// PrintResponse - ответ на запрос печати
type PrintResponse struct {
	UUID      string // UUID задания на печать
	URL       string // URL для скачивания PDF (доступен после готовности)
	Status    string // Статус: "ACCEPTED", "PROCESSING", "READY", "INVALID"
	CreatedAt string // Дата создания задания
}

// ========================
// Location Reference (Cities/Regions)
// ========================

// CitiesRequest - запрос на получение списка городов
type CitiesRequest struct {
	CountryCode *string // Код страны (ISO 3166-1 alpha-2)
	RegionCode  *int    // Код региона
	FiasGUID    *string // ФИАС код
	PostalCode  *string // Почтовый индекс
	Code        *int    // Код населенного пункта СДЭК
	City        *string // Название города (поиск)
	Size        *int    // Количество результатов (по умолчанию 1000)
	Page        *int    // Номер страницы
}

// City - город из справочника СДЭК
type City struct {
	Code          int     // Код населенного пункта СДЭК
	City          string  // Название города
	FiasGUID      *string // Уникальный идентификатор ФИАС
	Region        string  // Регион
	RegionCode    int     // Код региона
	Country       string  // Страна
	CountryCode   string  // Код страны
	Latitude      float64 // Широта
	Longitude     float64 // Долгота
	TimeZone      string  // Часовой пояс
	PaymentLimit  float64 // Ограничение оплаты наличными
	PostalCodes   []string // Почтовые индексы
}

// RegionsRequest - запрос на получение списка регионов
type RegionsRequest struct {
	CountryCode *string // Код страны (ISO 3166-1 alpha-2)
	RegionCode  *int    // Код региона
	Region      *string // Название региона (поиск)
	Size        *int    // Количество результатов
	Page        *int    // Номер страницы
}

// Region - регион из справочника СДЭК
type Region struct {
	Code        int     // Код региона
	Region      string  // Название региона
	Country     string  // Страна
	CountryCode string  // Код страны
	FiasGUID    *string // ФИАС код региона
}

// ========================
// Intake (Заявка на забор)
// ========================

// IntakeRequest - запрос на создание заявки на забор груза
type IntakeRequest struct {
	IntakeDate   string          // Дата ожидаемого забора (ISO 8601: YYYY-MM-DD)
	IntakeTimeFrom string        // Время начала ожидания (HH:MM)
	IntakeTimeTo   string        // Время окончания ожидания (HH:MM)
	LunchTimeFrom  *string       // Время начала обеда (HH:MM)
	LunchTimeTo    *string       // Время окончания обеда (HH:MM)
	Comment        *string       // Комментарий
	Sender         Contact       // Отправитель
	FromLocation   Location      // Адрес забора
	NeedCall       *bool         // Нужен ли звонок
	Orders         []IntakeOrder // Список заказов для забора
}

// IntakeOrder - заказ в заявке на забор
type IntakeOrder struct {
	OrderUUID string // UUID заказа
}

// IntakeResponse - ответ при создании заявки на забор
type IntakeResponse struct {
	UUID       string // UUID заявки
	Number     string // Номер заявки СДЭК
	IntakeDate string // Дата забора
	Status     string // Статус заявки
	CreatedAt  string // Дата создания
}

// IntakeInfo - информация о заявке на забор
type IntakeInfo struct {
	UUID           string          // UUID заявки
	Number         string          // Номер заявки
	IntakeDate     string          // Дата забора
	IntakeTimeFrom string          // Время начала
	IntakeTimeTo   string          // Время окончания
	Status         string          // Статус
	Sender         Contact         // Отправитель
	FromLocation   Location        // Адрес забора
	Orders         []IntakeOrder   // Заказы
	CreatedAt      string          // Дата создания
}

// ========================
// Webhooks
// ========================

// Типы событий webhook
const (
	WebhookTypeOrderStatus         = "ORDER_STATUS"          // Изменение статуса заказа
	WebhookTypeOrderModified       = "ORDER_MODIFIED"        // Изменение заказа
	WebhookTypePrintForm           = "PRINT_FORM"            // Готовность печатной формы
	WebhookTypeReceipt             = "RECEIPT"               // Квитанция
	WebhookTypePrealertClosed      = "PREALERT_CLOSED"       // Закрытие преалерта
	WebhookTypeAccompanyingWaybill = "ACCOMPANYING_WAYBILL"  // Информация о транспорте
	WebhookTypeOfficeAvailability  = "OFFICE_AVAILABILITY"   // Доступность офиса
	WebhookTypeDelivAgreement      = "DELIV_AGREEMENT"       // Договоренность о доставке
	WebhookTypeDelivProblem        = "DELIV_PROBLEM"         // Проблемы доставки
	WebhookTypeCourierInfo         = "COURIER_INFO"          // Информация о курьере
)

// WebhookRequest - запрос на создание webhook
type WebhookRequest struct {
	URL  string // URL для получения уведомлений (обязательно)
	Type string // Тип события (обязательно): ORDER_STATUS, PRINT_FORM, etc.
}

// WebhookResponse - ответ при создании webhook
type WebhookResponse struct {
	UUID string // UUID созданного webhook
}

// Webhook - информация о webhook
type Webhook struct {
	UUID string // UUID webhook
	URL  string // URL для уведомлений
	Type string // Тип события
}
