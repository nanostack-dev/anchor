// Package anchorsdk is the product-facing SDK for Anchor.
//
// It wraps the generated OpenAPI client ([nanoclient]) with a fluent, typed
// surface for the operations a product backend performs at runtime: sending
// transactional email, managing its organizations and their members, issuing
// organization API keys, and resolving credentials.
//
// # Scope
//
// This SDK covers what a product *uses*, authenticated with a product API key.
// It deliberately does not cover platform administration — creating products,
// managing platform users and invitations, or maintaining the permission and
// role catalog. Those are operated through the admin UI and Terraform, and
// remain available on the generated client via [Client.Raw].
//
// # Usage
//
//	c, err := anchorsdk.New(anchorsdk.Config{
//	    BaseURL:       "https://anchor.example.com",
//	    ProductID:     "prd_3iXYZ",
//	    ProductAPIKey: os.Getenv("ANCHOR_PRODUCT_API_KEY"),
//	})
//
//	err = c.Email().
//	    Template("welcome").
//	    To("new.user@company.com").
//	    Var("name", "Bob").
//	    Send(ctx)
//
//	org, err := c.Organizations().Create("Acme").
//	    Description("Leading provider of innovative solutions").
//	    Do(ctx)
//
//	members, err := c.Organization(org.Id).Members().List(ctx)
//
// The product ID is bound once at construction, so it never appears at a call
// site. Every operation that Anchor scopes to a product is reached without
// repeating it.
//
// # Dependencies
//
// This package depends only on the standard library and the generated client.
// It deliberately does not import nanostack-framework: the SDK is consumed by
// callers outside the Nanostack stack, and coupling its version to the
// framework's would force them to upgrade in lockstep.
//
// # Errors
//
// Every method returns *[Error] on a non-2xx response, carrying the status
// code, the operation name, and Anchor's structured error details. Classify
// with [errors.Is] against [ErrNotFound], [ErrForbidden], [ErrUnauthorized],
// [ErrConflict], [ErrInvalid], or [ErrPermanent].
package anchorsdk
