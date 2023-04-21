package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/melbahja/goph"
)

var (
	_ resource.Resource                = &domainResource{}
	_ resource.ResourceWithConfigure   = &domainResource{}
	_ resource.ResourceWithImportState = &domainResource{}
)

func NewDomainResource() resource.Resource {
	return &domainResource{}
}

type domainResource struct {
	client *goph.Client
}

type domainResourceModel struct {
	AppName types.String `tfsdk:"app_name"`
	Domain  types.String `tfsdk:"domain"`
}

// Metadata returns the resource type name.
func (r *domainResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

// Configure adds the provider configured client to the resource.
func (r *domainResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	//nolint:forcetypeassert
	r.client = req.ProviderData.(*goph.Client)
}

// Schema defines the schema for the resource.
func (r *domainResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"app_name": schema.StringAttribute{
				Required: true,
			},
			"domain": schema.StringAttribute{
				Required: true,
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *domainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state domainResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Read domains
	stdout, _, err := run(ctx, r.client, fmt.Sprintf("domains:report %s", state.AppName.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to read domains",
			"Unable to read domains. "+err.Error(),
		)
		return
	}

	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	found := false
	for _, line := range lines {
		parts := strings.Split(line, ":")
		key := strings.TrimSpace(parts[0])
		if key == "Domains app vhosts" {
			domainList := strings.TrimSpace(parts[1])
			if domainList != "" {
				for _, domain := range strings.Split(domainList, " ") {
					if domain == state.Domain.ValueString() {
						found = true
						break
					}
				}
			}
			break
		}
	}

	if !found {
		resp.Diagnostics.AddError("Domain not found", "Domain not found")
		return
	}

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Create creates the resource and sets the initial Terraform state.
func (r *domainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan domainResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Add domain
	_, _, err := run(ctx, r.client, fmt.Sprintf("domains:add %s %s", plan.AppName.ValueString(), plan.Domain.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create domain",
			"Unable to create domain. "+err.Error(),
		)
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *domainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan domainResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state domainResourceModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.AppName.ValueString() != state.AppName.ValueString() {
		resp.Diagnostics.AddAttributeError(path.Root("app_name"), "Unable to change app name", "Unable to change app name")
		return
	}

	if plan.Domain.ValueString() != state.Domain.ValueString() {
		_, _, err := run(ctx, r.client, fmt.Sprintf("domains:remove %s %s", state.AppName.ValueString(), state.Domain.ValueString()))
		if err != nil {
			resp.Diagnostics.AddError(
				"Unable to create domains",
				"Unable to create domains. "+err.Error(),
			)
			return
		}

		// Add domain
		_, _, err = run(ctx, r.client, fmt.Sprintf("domains:add %s %s", plan.AppName.ValueString(), plan.Domain.ValueString()))
		if err != nil {
			resp.Diagnostics.AddError(
				"Unable to create domain",
				"Unable to create domain. "+err.Error(),
			)
			return
		}
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *domainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state domainResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Clear domains
	_, _, err := run(ctx, r.client, fmt.Sprintf("domains:remove %s %s", state.AppName.ValueString(), state.Domain.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to delete domain",
			"Unable to delete domain. "+err.Error(),
		)
		return
	}
}

func (r *domainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, " ")
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("app_name"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("domain"), parts[1])...)
}
