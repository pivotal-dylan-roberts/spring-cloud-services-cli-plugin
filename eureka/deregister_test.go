/*
 * Copyright 2016-2017 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package eureka_test

import (
	//"github.com/pivotal-cf/spring-cloud-services-cli-plugin/eureka"

	"bytes"
	"errors"

	"code.cloudfoundry.org/cli/plugin/models"
	"code.cloudfoundry.org/cli/plugin/pluginfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/spring-cloud-services-cli-plugin/eureka"
	"github.com/pivotal-cf/spring-cloud-services-cli-plugin/httpclient"
	"github.com/pivotal-cf/spring-cloud-services-cli-plugin/httpclient/httpclientfakes"
)

var _ = Describe("Deregister", func() {

	var (
		fakeCliConnection *pluginfakes.FakeCliConnection
		fakeAuthClient    *httpclientfakes.FakeAuthenticatedClient
		fakeResolver      func(dashboardUrl string, accessToken string, authClient httpclient.AuthenticatedClient) (string, error)
		getServiceModel   plugin_models.GetService_Model
		output            string
		err               error
	)

	BeforeEach(func() {
		fakeCliConnection = &pluginfakes.FakeCliConnection{}
		fakeAuthClient = &httpclientfakes.FakeAuthenticatedClient{}
		fakeAuthClient.DoAuthenticatedGetReturns(bytes.NewBufferString("https://fake.com"), nil)
		fakeResolver = func(dashboardUrl string, accessToken string, authClient httpclient.AuthenticatedClient) (string, error) {
			return "https://eureka-dashboard-url/", nil
		}
	})

	JustBeforeEach(func() {
		output, err = eureka.DeregisterWithResolver(fakeCliConnection, "some-service-registry", "some-cf-app", fakeAuthClient, fakeResolver)
	})

	Context("when the service is not found", func() {
		BeforeEach(func() {
			fakeCliConnection.GetServiceReturns(getServiceModel, errors.New("some error"))
		})

		It("should return a suitable error", func() {
			Expect(fakeCliConnection.GetServiceCallCount()).To(Equal(1))
			Expect(fakeCliConnection.GetServiceArgsForCall(0)).To(Equal("some-service-registry"))
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("Service registry instance not found: some error"))
		})
	})

	Context("when the service is found", func() {
		BeforeEach(func() {
			getServiceModel.DashboardUrl = "https://spring-cloud-broker.some.host.name/x/y/z/some-guid"
			fakeCliConnection.GetServiceReturns(getServiceModel, nil)
		})

		Context("but the access token is not available", func() {
			var accessTokenCallCount int

			BeforeEach(func() {
				accessTokenCallCount = 0
				fakeCliConnection.AccessTokenStub = func() (string, error) {
					accessTokenCallCount++
					return "", errors.New("some access token error")
				}
			})

			It("should return a suitable error", func() {
				Expect(accessTokenCallCount).To(Equal(1))
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("Access token not available: some access token error"))
			})
		})

		Context("and the access token is available", func() {
			var accessTokenCallCount int

			BeforeEach(func() {
				accessTokenCallCount = 0
				fakeCliConnection.AccessTokenStub = func() (string, error) {
					accessTokenCallCount++
					return "someaccesstoken", nil
				}
			})

			Context("but the eureka dashboard URL cannot be resolved", func() {
				BeforeEach(func() {
					fakeResolver = func(dashboardUrl string, accessToken string, authClient httpclient.AuthenticatedClient) (string, error) {
						return "", errors.New("resolution error")
					}
				})

				It("should return a suitable error", func() {
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError("Error obtaining service registry dashboard URL: resolution error"))
				})
			})

			Context("the cf app can be resolved", func() {

				BeforeEach(func() {
					fakeResolver = func(dashboardUrl string, accessToken string, authClient httpclient.AuthenticatedClient) (string, error) {
						return "https://spring-cloud-broker.some.host.name/x/y/z/some-guid", nil
					}

					fakeCliConnection.GetAppsStub = func() ([]plugin_models.GetAppsModel, error) {
						apps := []plugin_models.GetAppsModel{}
						app1 := plugin_models.GetAppsModel{
							Name: "some-cf-app",
							Guid: "062bd505-8b19-44ca-4451-4a932932143a",
						}
						return append(apps, app1), nil
					}

					fakeAuthClient.DoAuthenticatedGetReturns(bytes.NewBufferString(`
						{
						   "applications":{
						      "application":[
							 {
							    "instance":[
							       {
								  "app":"APP-1",
								  "status":"UP",
								  "metadata":{
								     "zone":"zone-a",
								     "cfAppGuid":"062bd505-8b19-44ca-4451-4a932932143a",
								     "cfInstanceIndex":"2"
								  }
							       }
							    ]
							 }
						      ]
						   }
						}`), nil)

				})

				It("should successfully deregister the service", func() {
					Expect(fakeAuthClient.DoAuthenticatedDeleteCallCount()).To(Equal(1))
				})

				Context("but only two out of three eureka instance names can be resolved", func() {

					BeforeEach(func() {

						fakeAuthClient.DoAuthenticatedGetReturns(bytes.NewBufferString(`
						{
						   "applications":{
						      "application":[
							 {
							    "instance":[
							       {
								  "app":"APP-1",
								  "status":"UP",
								  "metadata":{
								     "zone":"zone-a",
								     "cfAppGuid":"062bd505-8b19-44ca-4451-4a932932143a",
								     "cfInstanceIndex":"1"
								  }
							       },
							       }							       {
								  "app":"APP-2",
								  "status":"UNKNOWN",
								  "metadata":{
								     "zone":"zone-a",
								     "cfInstanceIndex":"2"
								  }
							       },
							       {
								  "app":"APP-3",
								  "status":"UP",
								  "metadata":{
								     "zone":"zone-a",
								     "cfAppGuid":"162bd505-8b19-44ca-4451-4a932932143a",
								     "cfInstanceIndex":"3"
								  }
							       },
							    ]
							 }
						      ]
						   }
						}`), nil)

						It("should not deregister the service with a missing guid", func() {
							Expect(fakeAuthClient.DoAuthenticatedDeleteCallCount()).To(Equal(2))
						})
					})
				})

				Context("but the cf app name cannot be found", func() {

					BeforeEach(func() {
						fakeCliConnection.GetAppsStub = func() ([]plugin_models.GetAppsModel, error) {
							apps := []plugin_models.GetAppsModel{}
							app1 := plugin_models.GetAppsModel{
								Name: "unknown-app",
								Guid: "062bd505-8b19-44ca-4451-4a932932143a",
							}
							return append(apps, app1), nil
						}
					})

					It("should return a suitable error", func() {
						Expect(err).To(HaveOccurred())
						Expect(err).To(MatchError("cf app name some-cf-app not found"))
					})
				})
			})
		})
	})
})
