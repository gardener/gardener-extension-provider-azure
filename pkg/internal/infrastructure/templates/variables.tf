variable "SUBSCRIPTION_ID" {
  description = "Azure subscription id of technical user"
  type        = string
}

variable "TENANT_ID" {
  description = "Azure tenant id of technical user"
  type        = string
}

variable "CLIENT_ID" {
  description = "Azure client id of technical user"
  type        = string
}

variable "CLIENT_SECRET" {
  description = "Azure client secret of technical user"
  type        = string
  default     = ""
}
