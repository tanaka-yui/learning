Rails.application.routes.draw do
  get "health", to: "heavy#health"
  get "heavy", to: "heavy#heavy"
end
